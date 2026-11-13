package tokenissuer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/github"
	"github.com/tmknom/gates/internal/revocation"
)

const (
	// appJWTMarker は、KMS Signer のテストダブルが返す GitHub App JWT で、要求とエラー文言での扱いを確かめる目印である。
	appJWTMarker = "app-jwt-marker"
	// responseMarker は、応答本文がエラー文言に漏れていないことを確かめる目印である。
	responseMarker = "response-body-marker"
	// revocationRequestID は、失効の応答の GitHub の request ID で、発行応答の request ID と見分ける。
	revocationRequestID = "REVO:1234:5678:9ABC:DEF0"

	installationPath = "GET /users/" + allowedOwnerName + "/installation"
	issuancePath     = "POST /app/installations/12345678/access_tokens"
	revocationPath   = "DELETE /installation/token"
)

// signerStub は KMS Signer のテストダブルである。
// KMS Signer の作った GitHub App JWT を GitHub Client が送ることは、KMS のテストダブルを持つ KMS Signer の package のテストで確かめる。
type signerStub struct {
	err   error
	calls int
}

func (s *signerStub) Sign(context.Context) (github.AppJWT, error) {
	s.calls++
	if s.err != nil {
		return github.AppJWT{}, s.err
	}
	return github.NewAppJWT(appJWTMarker), nil
}

// gitHubStub は GitHub API の代わりに応答する TLS サーバーで、受け取った要求の Authorization を経路ごとに記録する。
type gitHubStub struct {
	server *httptest.Server
	mu     sync.Mutex
	// authorizations は、経路ごとに受け取った Authorization を受け取った順に持つ
	authorizations map[string][]string
	// revocationContexts は、GitHub Client が失効の要求に付けた ctx を、送ろうとした順に持つ。サーバーに届かなかった要求も含む
	revocationContexts []context.Context
}

// roundTripperFunc は、GitHub Client の要求を、サーバーへ送る前に観測するための Transport である。
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

// newGitHubStub は、Installation の取得、発行、失効の経路に、記録した応答を返すハンドラを置く。handlers は経路ごとにハンドラを差し替える。
func newGitHubStub(t *testing.T, handlers map[string]http.HandlerFunc) *gitHubStub {
	t.Helper()
	routes := map[string]http.HandlerFunc{
		installationPath: respond(http.StatusOK, requestID, marshal(t, map[string]any{"id": installationID, "account": map[string]any{"id": allowedOwnerID}, "suspended_at": nil})),
		issuancePath:     respond(http.StatusCreated, requestID, marshal(t, recordedIssuance())),
		revocationPath:   respond(http.StatusNoContent, revocationRequestID, ""),
	}
	maps.Copy(routes, handlers)
	stub := &gitHubStub{authorizations: map[string][]string{}}
	mux := http.NewServeMux()
	for pattern, handler := range routes {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			stub.mu.Lock()
			stub.authorizations[pattern] = append(stub.authorizations[pattern], r.Header.Get("Authorization"))
			stub.mu.Unlock()
			handler(w, r)
		})
	}
	stub.server = httptest.NewUnstartedServer(mux)
	// 応答を待つハンドラで、サーバーのログがテストの出力に混ざらないようにする
	stub.server.Config.ErrorLog = log.New(io.Discard, "", 0)
	stub.server.StartTLS()
	t.Cleanup(stub.server.Close)
	return stub
}

func (s *gitHubStub) client(t *testing.T) *github.Client {
	t.Helper()
	apiHost, err := url.Parse(s.server.URL)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	base := s.server.Client().Transport
	transport := roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodDelete {
			s.mu.Lock()
			s.revocationContexts = append(s.revocationContexts, request.Context())
			s.mu.Unlock()
		}
		return base.RoundTrip(request)
	})
	return github.NewClient(github.WithAPIHost(apiHost), github.WithTransport(transport))
}

// revocationContext は、GitHub Client が最初の失効の要求に付けた ctx を返す。
func (s *gitHubStub) revocationContext(t *testing.T) context.Context {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.revocationContexts) == 0 {
		t.Fatal("GitHub Client did not try to send a revocation request")
	}
	return s.revocationContexts[0]
}

// received は、経路が受け取った要求の Authorization を返す。
func (s *gitHubStub) received(pattern string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.authorizations[pattern]
}

func respond(status int, gitHubRequestID, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-GitHub-Request-Id", gitHubRequestID)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

// grantRecorder は、Lambda Handler が渡す発行許可の確定の関数の代わりに、受け取った Installation を記録して grant を通す。
type grantRecorder struct {
	t       *testing.T
	err     error
	granted []authz.Installation
}

func (g *grantRecorder) grant(installation authz.Installation) (authz.TokenGrant, error) {
	g.granted = append(g.granted, installation)
	if g.err != nil {
		return authz.TokenGrant{}, g.err
	}
	return grant(g.t, installation)
}

func newTestTokenIssuer(t *testing.T, signer *signerStub, stub *gitHubStub) *TokenIssuer {
	t.Helper()
	return NewTokenIssuer(signer, stub.client(t), allowedOwner(t), func() time.Time { return now })
}

// assertHidesValues は、エラー文言に GitHub App JWT、トークン、応答本文が入らないことを確かめる。
func assertHidesValues(t *testing.T, err error) {
	t.Helper()
	for _, value := range []string{appJWTMarker, tokenMarker, responseMarker} {
		if strings.Contains(err.Error(), value) {
			t.Errorf("error message %q contains %q", err.Error(), value)
		}
	}
}

func TestTokenIssuer_IssueInstallationToken_ReturnsVerifiedInstallationToken(t *testing.T) {
	signer := &signerStub{}
	stub := newGitHubStub(t, nil)
	recorder := &grantRecorder{t: t}

	got, err := newTestTokenIssuer(t, signer, stub).IssueInstallationToken(t.Context(), recorder.grant)

	if err != nil {
		t.Fatalf("IssueInstallationToken returned error: %v", err)
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
}

func TestTokenIssuer_IssueInstallationToken_PassesResolvedInstallationToGrant(t *testing.T) {
	recorder := &grantRecorder{t: t}

	if _, err := newTestTokenIssuer(t, &signerStub{}, newGitHubStub(t, nil)).IssueInstallationToken(t.Context(), recorder.grant); err != nil {
		t.Fatalf("IssueInstallationToken returned error: %v", err)
	}

	want, err := authz.NewInstallation(installationID, allowedOwnerID, time.Time{})
	if err != nil {
		t.Fatalf("NewInstallation returned error: %v", err)
	}
	if !reflect.DeepEqual(recorder.granted, []authz.Installation{want}) {
		t.Errorf("grant received %+v, want [%+v]", recorder.granted, want)
	}
}

func TestTokenIssuer_IssueInstallationToken_ReturnsGrantFailureWithoutIssuance(t *testing.T) {
	for _, kind := range []error{authz.ErrAuthorizationDenied, authz.ErrInstallationUnavailable} {
		t.Run(kind.Error()+" is returned as is", func(t *testing.T) {
			want := fmt.Errorf("%w: condition", kind)
			stub := newGitHubStub(t, nil)

			got, err := newTestTokenIssuer(t, &signerStub{}, stub).IssueInstallationToken(t.Context(), (&grantRecorder{t: t, err: want}).grant)

			if !errors.Is(err, want) {
				t.Errorf("IssueInstallationToken error = %v, want %v", err, want)
			}
			if !reflect.ValueOf(got).IsZero() {
				t.Error("IssueInstallationToken returned non-zero InstallationToken with error")
			}
			if n := len(stub.received(issuancePath)); n != 0 {
				t.Errorf("issuance received %d requests, want 0", n)
			}
		})
	}
}

//trace:checks AC-12
func TestTokenIssuer_IssueInstallationToken_RevokesUnverifiedToken(t *testing.T) {
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

			got, err := newTestTokenIssuer(t, &signerStub{}, stub).IssueInstallationToken(t.Context(), (&grantRecorder{t: t}).grant)

			if !errors.Is(err, ErrVerificationFailure) {
				t.Fatalf("IssueInstallationToken error = %v, want %v", err, ErrVerificationFailure)
			}
			// 失効の失敗は失敗の種別ではなく、照合失敗をほかの種別として判定させない
			for _, other := range []error{github.ErrTransientFailure, github.ErrIssuanceRejected, authz.ErrInstallationUnavailable} {
				if errors.Is(err, other) {
					t.Errorf("IssueInstallationToken error = %v, also matches %v", err, other)
				}
			}
			failure, ok := errors.AsType[*VerificationFailure](err)
			if !ok {
				t.Fatalf("IssueInstallationToken error = %v, want *VerificationFailure", err)
			}
			if failure.RevocationOutcome() != tt.want {
				t.Errorf("RevocationOutcome() = %v, want %v", failure.RevocationOutcome(), tt.want)
			}
			if !reflect.ValueOf(got).IsZero() {
				t.Error("IssueInstallationToken returned non-zero InstallationToken with error")
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

//trace:checks AC-12 AC-29
func TestTokenIssuer_IssueInstallationToken_DoesNotRevokeWithoutToken(t *testing.T) {
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

			_, err := newTestTokenIssuer(t, &signerStub{}, stub).IssueInstallationToken(t.Context(), (&grantRecorder{t: t}).grant)

			if !errors.Is(err, tt.want) {
				t.Fatalf("IssueInstallationToken error = %v, want %v", err, tt.want)
			}
			// 照合失敗は失効を試みなかったことを持ち、ほかの種別は失効の結果を持たない
			failure, ok := errors.AsType[*VerificationFailure](err)
			if want := errors.Is(tt.want, ErrVerificationFailure); ok != want {
				t.Errorf("IssueInstallationToken error is *VerificationFailure = %v, want %v", ok, want)
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

// unverifiedIssuance は、トークン文字列を持ち、repository_selection が all のため照合に失敗する発行応答を返す経路である。
func unverifiedIssuance(t *testing.T) map[string]http.HandlerFunc {
	t.Helper()
	response := recordedIssuance()
	response["repository_selection"] = "all"
	return map[string]http.HandlerFunc{issuancePath: respond(http.StatusCreated, requestID, marshal(t, response))}
}

// 関数の残り時間は要求の ctx の期限から読み、失効は応答を書くための 1 秒を残して打ち切る（Lambda Design の Call Time Budget）。
func TestTokenIssuer_IssueInstallationToken_RevocationEndsOneSecondBeforeRequestDeadline(t *testing.T) {
	stub := newGitHubStub(t, unverifiedIssuance(t))
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	requestDeadline, _ := ctx.Deadline()

	_, err := newTestTokenIssuer(t, &signerStub{}, stub).IssueInstallationToken(ctx, (&grantRecorder{t: t}).grant)

	if !errors.Is(err, ErrVerificationFailure) {
		t.Fatalf("IssueInstallationToken error = %v, want %v", err, ErrVerificationFailure)
	}
	got, ok := stub.revocationContext(t).Deadline()
	if want := requestDeadline.Add(-time.Second); !ok || !got.Equal(want) {
		t.Errorf("revocation request deadline = %v (set %v), want %v", got, ok, want)
	}
}

// 発行の要求を送った後に要求の ctx の期限が来ると、応答を受け取れずトークン文字列がないため失効を試みない。
//
//trace:checks AC-30
func TestTokenIssuer_IssueInstallationToken_DoesNotRevokeAfterIssuanceDeadline(t *testing.T) {
	waitForCancel := func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	}
	stub := newGitHubStub(t, map[string]http.HandlerFunc{issuancePath: waitForCancel})
	// Installation の解決を終えてから発行の応答を待つ間に期限が来る長さにする
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	_, err := newTestTokenIssuer(t, &signerStub{}, stub).IssueInstallationToken(ctx, (&grantRecorder{t: t}).grant)

	if !errors.Is(err, github.ErrTransientFailure) {
		t.Fatalf("IssueInstallationToken error = %v, want %v", err, github.ErrTransientFailure)
	}
	if _, ok := errors.AsType[*VerificationFailure](err); ok {
		t.Errorf("IssueInstallationToken error = %v, want no *VerificationFailure", err)
	}
	if n := len(stub.received(revocationPath)); n != 0 {
		t.Errorf("revocation received %d requests, want 0", n)
	}
}

// 失効は要求の ctx の中断を引き継がない。入口から通すと、発行応答を受け取った後で要求の ctx を中断させる準備が要るため、段階の関数で確かめる。
func TestRevocationContext_IgnoresCancellationOfRequestContext(t *testing.T) {
	withDeadline := func(ctx context.Context) (context.Context, context.CancelFunc) {
		return context.WithTimeout(ctx, 30*time.Second)
	}
	tests := []struct {
		name         string
		newContext   func(context.Context) (context.Context, context.CancelFunc)
		withDeadline bool
	}{
		{name: "request context without deadline", newContext: context.WithCancel, withDeadline: false},
		{name: "request context with deadline", newContext: withDeadline, withDeadline: true},
	}
	for _, tt := range tests {
		t.Run(tt.name+" is not done after cancellation", func(t *testing.T) {
			ctx, cancel := tt.newContext(t.Context())
			cancel()

			got, cancelRevocation := revocationContext(ctx)
			defer cancelRevocation()

			if err := got.Err(); err != nil {
				t.Errorf("revocation context Err() = %v, want nil", err)
			}
			if _, ok := got.Deadline(); ok != tt.withDeadline {
				t.Errorf("revocation context has deadline = %v, want %v", ok, tt.withDeadline)
			}
		})
	}
}

//trace:checks AC-13
func TestTokenIssuer_IssueInstallationToken_SignFailureCallsNoGitHubAPI(t *testing.T) {
	want := errors.New("kms sign failed")
	stub := newGitHubStub(t, nil)
	recorder := &grantRecorder{t: t}

	_, err := newTestTokenIssuer(t, &signerStub{err: want}, stub).IssueInstallationToken(t.Context(), recorder.grant)

	if !errors.Is(err, want) {
		t.Fatalf("IssueInstallationToken error = %v, want %v", err, want)
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

func TestTokenIssuer_IssueInstallationToken_ResolutionFailureCallsNeitherGrantNorIssuance(t *testing.T) {
	stub := newGitHubStub(t, map[string]http.HandlerFunc{installationPath: respond(http.StatusNotFound, requestID, responseMarker)})
	recorder := &grantRecorder{t: t}

	_, err := newTestTokenIssuer(t, &signerStub{}, stub).IssueInstallationToken(t.Context(), recorder.grant)

	if !errors.Is(err, authz.ErrInstallationUnavailable) {
		t.Fatalf("IssueInstallationToken error = %v, want %v", err, authz.ErrInstallationUnavailable)
	}
	if len(recorder.granted) != 0 {
		t.Errorf("grant was called %d times, want 0", len(recorder.granted))
	}
	if n := len(stub.received(issuancePath)); n != 0 {
		t.Errorf("issuance received %d requests, want 0", n)
	}
	assertHidesValues(t, err)
}

func TestTokenIssuer_IssueInstallationToken_NamesGitHubStageInError(t *testing.T) {
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
			_, err := newTestTokenIssuer(t, &signerStub{}, newGitHubStub(t, tt.handlers)).IssueInstallationToken(t.Context(), (&grantRecorder{t: t}).grant)

			if err == nil || !strings.HasPrefix(err.Error(), tt.want) {
				t.Errorf("IssueInstallationToken error = %v, want prefix %q", err, tt.want)
			}
		})
	}
}

func TestNewTokenIssuer_ZeroAllowedOwnerPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewTokenIssuer did not panic for the zero AllowedOwner")
		}
	}()

	_ = NewTokenIssuer(&signerStub{}, github.NewClient(), authz.AllowedOwner{}, func() time.Time { return now })
}
