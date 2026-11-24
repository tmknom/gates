package tokenissuer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/github"
	"github.com/tmknom/gates/internal/protected"
	"github.com/tmknom/gates/internal/repositorypolicy"
	"github.com/tmknom/gates/internal/revocation"
)

// grantRecorder は、リポジトリポリシーを適用しないデプロイで Lambda Handler が渡す Authorization の代わりに、Grant が受け取った Installation を記録して grant を通す。
// 適用しないデプロイでは使わない操作が呼ばれたら、テストを失敗させる。
type grantRecorder struct {
	t       *testing.T
	err     error
	granted []authz.Installation
}

//trace:checks AC-AUTHZ-017
func TestTokenIssuer_IssueInstallationAccessToken_ReturnsReturnableIssuanceResult(t *testing.T) {
	signer := &signerStub{}
	stub := newGitHubStub(t, nil)
	recorder := &grantRecorder{t: t}

	got, outcome, err := newTestTokenIssuer(t, signer, stub).IssueInstallationAccessToken(t.Context(), recorder)

	if err != nil {
		t.Fatalf("IssueInstallationAccessToken returned error: %v", err)
	}
	if want := time.Date(2026, 11, 3, 12, 18, 48, 0, time.UTC); !got.ExpiresAt().Equal(want) {
		t.Errorf("ExpiresAt() = %v, want %v", got.ExpiresAt(), want)
	}
	if signer.calls != 1 {
		t.Errorf("Sign calls = %d, want 1", signer.calls)
	}
	// Installation の解決と発行は、要求ごとに 1 回だけ作った GitHub App JWT を使う
	for _, pattern := range []string{installationPath, issuancePath} {
		if got := stub.received(pattern); !reflect.DeepEqual(got, []string{"Bearer " + appJWTMarker}) {
			t.Errorf("%s received Authorization %q, want [%q]", pattern, got, "Bearer "+appJWTMarker)
		}
	}
	if got := stub.received(revocationPath); len(got) != 0 {
		t.Errorf("revocation received %d requests, want 0", len(got))
	}
	// ポリシー読み取りトークンの発行 API を呼ばないため、その失効の結果を持たず、リポジトリポリシーも取得しない
	if !outcome.IsZero() {
		t.Errorf("policy read revocation outcome = %v, want the zero value", outcome)
	}
	if got := stub.received(repositoryPolicyPath); len(got) != 0 {
		t.Errorf("repository policy fetch received %d requests, want 0", len(got))
	}
}

func TestTokenIssuer_IssueInstallationAccessToken_PassesResolvedInstallationToGrant(t *testing.T) {
	recorder := &grantRecorder{t: t}

	if _, _, err := newTestTokenIssuer(t, &signerStub{}, newGitHubStub(t, nil)).IssueInstallationAccessToken(t.Context(), recorder); err != nil {
		t.Fatalf("IssueInstallationAccessToken returned error: %v", err)
	}

	want, err := authz.NewInstallation(installationID, allowedOwnerID, false)
	if err != nil {
		t.Fatalf("NewInstallation returned error: %v", err)
	}
	if !reflect.DeepEqual(recorder.granted, []authz.Installation{want}) {
		t.Errorf("grant received %+v, want [%+v]", recorder.granted, want)
	}
}

func TestTokenIssuer_IssueInstallationAccessToken_ReturnsGrantFailureWithoutIssuance(t *testing.T) {
	for _, kind := range []error{authz.ErrAuthorizationDenied, authz.ErrInstallationUnavailable} {
		t.Run(kind.Error()+" is returned as is", func(t *testing.T) {
			want := fmt.Errorf("%w: condition", kind)
			stub := newGitHubStub(t, nil)

			got, _, err := newTestTokenIssuer(t, &signerStub{}, stub).IssueInstallationAccessToken(t.Context(), &grantRecorder{t: t, err: want})

			if !errors.Is(err, want) {
				t.Errorf("IssueInstallationAccessToken error = %v, want %v", err, want)
			}
			if !reflect.ValueOf(got).IsZero() {
				t.Error("IssueInstallationAccessToken returned non-zero ReturnableIssuanceResult with error")
			}
			if n := len(stub.received(issuancePath)); n != 0 {
				t.Errorf("issuance received %d requests, want 0", n)
			}
		})
	}
}

//trace:checks AC-ISSUANCE-004
func TestTokenIssuer_IssueInstallationAccessToken_RevokesTokenAfterVerificationFailure(t *testing.T) {
	tests := []struct {
		name       string
		revocation http.HandlerFunc
		want       revocation.Outcome
		// reasonAdded は、失効の失敗の理由が照合失敗の文言に添えられることを表す。失効の応答の GitHub の request ID で見分ける
		reasonAdded bool
	}{
		{
			name:       "revocation with 204 is read as succeeded",
			revocation: respond(http.StatusNoContent, revocationRequestID, ""),
			want:       revocation.Succeeded,
		},
		{
			name:        "revocation with 500 is read as failed",
			revocation:  respond(http.StatusInternalServerError, revocationRequestID, responseMarker),
			want:        revocation.Failed,
			reasonAdded: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlers := unverifiedIssuance(t)
			handlers[revocationPath] = tt.revocation
			stub := newGitHubStub(t, handlers)

			got, _, err := newTestTokenIssuer(t, &signerStub{}, stub).IssueInstallationAccessToken(t.Context(), &grantRecorder{t: t})

			if !errors.Is(err, ErrVerificationFailure) {
				t.Fatalf("IssueInstallationAccessToken error = %v, want %v", err, ErrVerificationFailure)
			}
			// 失効の失敗は失敗の種別ではなく、照合失敗をほかの種別として判定させない
			for _, other := range []error{github.ErrTransientFailure, github.ErrIssuanceRejected, authz.ErrInstallationUnavailable} {
				if errors.Is(err, other) {
					t.Errorf("IssueInstallationAccessToken error = %v, also matches %v", err, other)
				}
			}
			failure, ok := errors.AsType[*VerificationFailure](err)
			if !ok {
				t.Fatalf("IssueInstallationAccessToken error = %v, want *VerificationFailure", err)
			}
			if failure.RevocationOutcome() != tt.want {
				t.Errorf("RevocationOutcome() = %v, want %v", failure.RevocationOutcome(), tt.want)
			}
			if !reflect.ValueOf(got).IsZero() {
				t.Error("IssueInstallationAccessToken returned non-zero ReturnableIssuanceResult with error")
			}
			if got := stub.received(revocationPath); !reflect.DeepEqual(got, []string{"Bearer " + tokenMarker}) {
				t.Errorf("revocation received Authorization %q, want [%q]", got, "Bearer "+tokenMarker)
			}
			message := err.Error()
			if !strings.Contains(message, "repository_selection is not selected") {
				t.Errorf("error message %q does not contain the unmet condition", message)
			}
			if got := strings.Contains(message, revocationRequestID); got != tt.reasonAdded {
				t.Errorf("error message %q contains the revocation reason = %v, want %v", message, got, tt.reasonAdded)
			}
			assertHidesValues(t, err)
		})
	}
}

//trace:checks AC-ISSUANCE-004 AC-OPS-008
func TestTokenIssuer_IssueInstallationAccessToken_DoesNotRevokeWithoutToken(t *testing.T) {
	tests := []struct {
		name     string
		issuance http.HandlerFunc
		want     error
	}{
		{name: "issuance result without token string returns ErrVerificationFailure", issuance: respond(http.StatusCreated, requestID, "not json "+responseMarker), want: ErrVerificationFailure},
		{name: "status 422 returns ErrIssuanceRejected", issuance: respond(http.StatusUnprocessableEntity, requestID, `{"token":"`+tokenMarker+`","message":"`+responseMarker+`"}`), want: github.ErrIssuanceRejected},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := newGitHubStub(t, map[string]http.HandlerFunc{issuancePath: tt.issuance})

			_, _, err := newTestTokenIssuer(t, &signerStub{}, stub).IssueInstallationAccessToken(t.Context(), &grantRecorder{t: t})

			if !errors.Is(err, tt.want) {
				t.Fatalf("IssueInstallationAccessToken error = %v, want %v", err, tt.want)
			}
			// 照合失敗は失効を試みなかったことを持ち、ほかの種別は失効の結果を持たない
			failure, ok := errors.AsType[*VerificationFailure](err)
			if want := errors.Is(tt.want, ErrVerificationFailure); ok != want {
				t.Errorf("IssueInstallationAccessToken error is *VerificationFailure = %v, want %v", ok, want)
			}
			if ok && failure.RevocationOutcome() != revocation.NotAttempted {
				t.Errorf("RevocationOutcome() = %v, want %v", failure.RevocationOutcome(), revocation.NotAttempted)
			}
			if n := len(stub.received(revocationPath)); n != 0 {
				t.Errorf("revocation received %d requests, want 0", n)
			}
			assertHidesValues(t, err)
		})
	}
}

// 関数の残り時間は要求の ctx の期限から読み、失効は応答を書くための 1 秒を残して打ち切る（Lambda Design の Call Time Budget）。
func TestTokenIssuer_IssueInstallationAccessToken_RevocationEndsOneSecondBeforeRequestDeadline(t *testing.T) {
	stub := newGitHubStub(t, unverifiedIssuance(t))
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	requestDeadline, _ := ctx.Deadline()

	_, _, err := newTestTokenIssuer(t, &signerStub{}, stub).IssueInstallationAccessToken(ctx, &grantRecorder{t: t})

	if !errors.Is(err, ErrVerificationFailure) {
		t.Fatalf("IssueInstallationAccessToken error = %v, want %v", err, ErrVerificationFailure)
	}
	got, ok := stub.revocationContext(t).Deadline()
	if want := requestDeadline.Add(-time.Second); !ok || !got.Equal(want) {
		t.Errorf("revocation request deadline = %v (set %v), want %v", got, ok, want)
	}
}

// 発行の要求を送った後に要求の ctx の期限が来ると、応答を受け取れずトークン文字列がないため失効を試みない。
//
//trace:checks AC-ISSUANCE-006
func TestTokenIssuer_IssueInstallationAccessToken_DoesNotRevokeAfterIssuanceDeadline(t *testing.T) {
	waitForCancel := func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	}
	stub := newGitHubStub(t, map[string]http.HandlerFunc{issuancePath: waitForCancel})
	// Installation の解決を終えてから発行の応答を待つ間に期限が来る長さにする
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	_, _, err := newTestTokenIssuer(t, &signerStub{}, stub).IssueInstallationAccessToken(ctx, &grantRecorder{t: t})

	if !errors.Is(err, github.ErrTransientFailure) {
		t.Fatalf("IssueInstallationAccessToken error = %v, want %v", err, github.ErrTransientFailure)
	}
	if _, ok := errors.AsType[*VerificationFailure](err); ok {
		t.Errorf("IssueInstallationAccessToken error = %v, want no *VerificationFailure", err)
	}
	if n := len(stub.received(revocationPath)); n != 0 {
		t.Errorf("revocation received %d requests, want 0", n)
	}
}

//trace:checks AC-QUALITY-001
func TestTokenIssuer_IssueInstallationAccessToken_SignFailureCallsNoGitHubAPI(t *testing.T) {
	want := errors.New("kms sign failed")
	stub := newGitHubStub(t, nil)
	recorder := &grantRecorder{t: t}

	_, _, err := newTestTokenIssuer(t, &signerStub{err: want}, stub).IssueInstallationAccessToken(t.Context(), recorder)

	if !errors.Is(err, want) {
		t.Fatalf("IssueInstallationAccessToken error = %v, want %v", err, want)
	}
	for _, pattern := range []string{installationPath, issuancePath, revocationPath} {
		if n := len(stub.received(pattern)); n != 0 {
			t.Errorf("%s received %d requests, want 0", pattern, n)
		}
	}
	if len(recorder.granted) != 0 {
		t.Errorf("grant was called %d times, want 0", len(recorder.granted))
	}
}

func TestTokenIssuer_IssueInstallationAccessToken_ResolutionFailureCallsNeitherGrantNorIssuance(t *testing.T) {
	stub := newGitHubStub(t, map[string]http.HandlerFunc{installationPath: respond(http.StatusNotFound, requestID, responseMarker)})
	recorder := &grantRecorder{t: t}

	_, _, err := newTestTokenIssuer(t, &signerStub{}, stub).IssueInstallationAccessToken(t.Context(), recorder)

	if !errors.Is(err, authz.ErrInstallationUnavailable) {
		t.Fatalf("IssueInstallationAccessToken error = %v, want %v", err, authz.ErrInstallationUnavailable)
	}
	if len(recorder.granted) != 0 {
		t.Errorf("grant was called %d times, want 0", len(recorder.granted))
	}
	if n := len(stub.received(issuancePath)); n != 0 {
		t.Errorf("issuance received %d requests, want 0", n)
	}
	assertHidesValues(t, err)
}

func TestTokenIssuer_IssueInstallationAccessToken_NamesGitHubStageInError(t *testing.T) {
	tests := []struct {
		name     string
		handlers map[string]http.HandlerFunc
		want     string
	}{
		{name: "installation resolution failure names resolve installation", handlers: map[string]http.HandlerFunc{installationPath: respond(http.StatusNotFound, requestID, "")}, want: "resolve installation: "},
		{name: "issuance failure names issue", handlers: map[string]http.HandlerFunc{issuancePath: respond(http.StatusNotFound, requestID, "")}, want: "issue: "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := newTestTokenIssuer(t, &signerStub{}, newGitHubStub(t, tt.handlers)).IssueInstallationAccessToken(t.Context(), &grantRecorder{t: t})

			if err == nil || !strings.HasPrefix(err.Error(), tt.want) {
				t.Errorf("IssueInstallationAccessToken error = %v, want prefix %q", err, tt.want)
			}
		})
	}
}

// 保護対象との食い違いも、ほかの照合失敗と同じ経路で失効を試みることを確かめる。失効の結果の読み方は、ほかの照合失敗のテストで確かめる
//
//trace:checks AC-AUTHZ-012 AC-ISSUANCE-004
func TestTokenIssuer_IssueInstallationAccessToken_RevokesTokenAfterProtectedRepositoryMismatch(t *testing.T) {
	stub := newGitHubStub(t, nil)
	tokenIssuer := NewTokenIssuer(&signerStub{}, stub.client(t), allowedOwner(t), protectedRepositoriesOf(t, strconv.Itoa(repositoryIDB), "old-name"), authz.RepositoryPolicyNotEnforced, func() time.Time { return now })

	got, _, err := tokenIssuer.IssueInstallationAccessToken(t.Context(), &grantRecorder{t: t})

	failure, ok := errors.AsType[*VerificationFailure](err)
	if !ok {
		t.Fatalf("IssueInstallationAccessToken error = %v, want *VerificationFailure", err)
	}
	if failure.RevocationOutcome() != revocation.Succeeded {
		t.Errorf("RevocationOutcome() = %v, want %v", failure.RevocationOutcome(), revocation.Succeeded)
	}
	if !reflect.ValueOf(got).IsZero() {
		t.Error("IssueInstallationAccessToken returned non-zero ReturnableIssuanceResult with error")
	}
	if !strings.Contains(err.Error(), "repositories[1]: protected repositories[") {
		t.Errorf("error message %q does not contain the unmet condition", err)
	}
}

func TestNewTokenIssuer_ZeroAllowedOwnerPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewTokenIssuer did not panic for the zero AllowedOwner")
		}
	}()

	_ = NewTokenIssuer(&signerStub{}, github.NewClient(), authz.AllowedOwner{}, noProtectedRepositories(t), authz.RepositoryPolicyNotEnforced, func() time.Time { return now })
}

func TestNewTokenIssuer_ZeroProtectedRepositoriesPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewTokenIssuer did not panic for the zero protected repositories")
		}
	}()

	_ = NewTokenIssuer(&signerStub{}, github.NewClient(), allowedOwner(t), protected.Repositories{}, authz.RepositoryPolicyNotEnforced, func() time.Time { return now })
}

//trace:checks AC-AUTHZ-017
func TestNewTokenIssuer_ZeroRepositoryPolicyEnforcementPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewTokenIssuer did not panic for the zero RepositoryPolicyEnforcement")
		}
	}()

	_ = NewTokenIssuer(&signerStub{}, github.NewClient(), allowedOwner(t), noProtectedRepositories(t), authz.RepositoryPolicyEnforcement{}, func() time.Time { return now })
}

func (g *grantRecorder) Grant(installation authz.Installation) (authz.TokenGrant, error) {
	g.granted = append(g.granted, installation)
	if g.err != nil {
		return authz.TokenGrant{}, g.err
	}
	return grant(g.t, installation, contentsRead)
}

func (g *grantRecorder) GrantPolicyRead(authz.Installation) (authz.PolicyReadGrant, error) {
	g.t.Error("GrantPolicyRead was called without repository policy enforcement")
	return authz.PolicyReadGrant{}, authz.ErrAuthorizationDenied
}

func (g *grantRecorder) GrantWithRepositoryPolicies(authz.Installation, []repositorypolicy.FetchResult) (authz.TokenGrant, error) {
	g.t.Error("GrantWithRepositoryPolicies was called without repository policy enforcement")
	return authz.TokenGrant{}, authz.ErrAuthorizationDenied
}

// unverifiedIssuance は、トークン文字列を持ち、repository_selection が all のため照合に失敗する発行応答を返す経路である。
func unverifiedIssuance(t *testing.T) map[string]http.HandlerFunc {
	t.Helper()
	response := recordedIssuance()
	response["repository_selection"] = "all"
	return map[string]http.HandlerFunc{issuancePath: respond(http.StatusCreated, requestID, marshal(t, response))}
}
