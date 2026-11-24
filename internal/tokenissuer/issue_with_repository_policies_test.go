package tokenissuer

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/github"
	"github.com/tmknom/gates/internal/repositorypolicy"
	"github.com/tmknom/gates/internal/revocation"
)

// repositoryPolicyGrantRecorder は、リポジトリポリシーを適用するデプロイで Lambda Handler が渡す Authorization の代わりに、受け取った値を記録して authz の確定関数を通す。
// 適用するデプロイでは使わない Grant が呼ばれたら、テストを失敗させる。
type repositoryPolicyGrantRecorder struct {
	t             *testing.T
	policyReadErr error
	grantErr      error
	// policyReadInstallations は、GrantPolicyRead が受け取った Installation を受け取った順に持つ
	policyReadInstallations []authz.Installation
	// grantInstallations と fetchResults は、GrantWithRepositoryPolicies が受け取った値を受け取った順に持つ
	grantInstallations []authz.Installation
	fetchResults       [][]repositorypolicy.FetchResult
}

// ポリシー読み取りトークンで読んだリポジトリポリシーで発行許可を確定し、利用者に返すトークンを返す。
// GitHub App JWT は 1 回だけ作り、ポリシー読み取りトークンは利用者に返すトークンの発行より前に失効させる。
//
//trace:checks AC-ISSUANCE-011 AC-AUTHZ-014
func TestTokenIssuer_IssueInstallationAccessToken_WithRepositoryPolicies_ReturnsTokenIssuedAfterReadingRepositoryPolicies(t *testing.T) {
	signer := &signerStub{}
	stub := newGitHubStub(t, enforcedRoutes(t))

	got, outcome, err := newEnforcedTestTokenIssuer(t, signer, stub).IssueInstallationAccessToken(t.Context(), &repositoryPolicyGrantRecorder{t: t})

	if err != nil {
		t.Fatalf("IssueInstallationAccessToken returned error: %v", err)
	}
	// トークン文字列は Lambda Handler だけが取り出せるため、ポリシー読み取りトークンと異なる実効権限で見分ける
	if got, want := maps.Collect(got.EffectivePermissions().Entries()), map[string]string{"contents": "read", "metadata": "read"}; !maps.Equal(got, want) {
		t.Errorf("effective permissions = %v, want %v of the token issued with the token grant", got, want)
	}
	if outcome != revocation.Succeeded {
		t.Errorf("policy read revocation outcome = %v, want %v", outcome, revocation.Succeeded)
	}
	if signer.calls != 1 {
		t.Errorf("Sign calls = %d, want 1", signer.calls)
	}
	want := []string{
		installationPath + " Bearer " + appJWTMarker,
		issuancePath + " Bearer " + appJWTMarker,
		repositoryPolicyPath + " Bearer " + policyReadTokenMarker,
		repositoryPolicyPath + " Bearer " + policyReadTokenMarker,
		revocationPath + " Bearer " + policyReadTokenMarker,
		issuancePath + " Bearer " + appJWTMarker,
	}
	if got := stub.receivedInOrder(); !reflect.DeepEqual(got, want) {
		t.Errorf("GitHub API received %q, want %q", got, want)
	}
}

// Authorization の 2 つの確定に、解決した Installation と、要求の対象の順に並べた取得の結果を渡す。
//
//trace:checks AC-AUTHZ-014
func TestTokenIssuer_IssueInstallationAccessToken_WithRepositoryPolicies_PassesInstallationAndFetchResultsToAuthorization(t *testing.T) {
	recorder := &repositoryPolicyGrantRecorder{t: t}

	if _, _, err := newEnforcedTestTokenIssuer(t, &signerStub{}, newGitHubStub(t, enforcedRoutes(t))).IssueInstallationAccessToken(t.Context(), recorder); err != nil {
		t.Fatalf("IssueInstallationAccessToken returned error: %v", err)
	}

	installation, err := authz.NewInstallation(installationID, allowedOwnerID, false)
	if err != nil {
		t.Fatalf("NewInstallation returned error: %v", err)
	}
	if want := []authz.Installation{installation}; !reflect.DeepEqual(recorder.policyReadInstallations, want) {
		t.Errorf("GrantPolicyRead received %+v, want %+v", recorder.policyReadInstallations, want)
	}
	if want := []authz.Installation{installation}; !reflect.DeepEqual(recorder.grantInstallations, want) {
		t.Errorf("GrantWithRepositoryPolicies received installations %+v, want %+v", recorder.grantInstallations, want)
	}
	want := [][]repositorypolicy.FetchResult{{
		repositorypolicy.Document(allowingDocumentOf("repo-b")),
		repositorypolicy.Document(allowingDocumentOf("Example-Repo")),
	}}
	if !reflect.DeepEqual(recorder.fetchResults, want) {
		t.Errorf("GrantWithRepositoryPolicies received fetch results %+v, want %+v", recorder.fetchResults, want)
	}
}

// ポリシー読み取り許可を確定できなければ、ポリシー読み取りトークンを発行しない。
//
//trace:checks AC-ISSUANCE-011
func TestTokenIssuer_IssueInstallationAccessToken_WithRepositoryPolicies_PolicyReadGrantFailureIssuesNoToken(t *testing.T) {
	for _, kind := range []error{authz.ErrAuthorizationDenied, authz.ErrInstallationUnavailable} {
		t.Run(kind.Error()+" is returned as is", func(t *testing.T) {
			want := fmt.Errorf("%w: condition", kind)
			stub := newGitHubStub(t, enforcedRoutes(t))
			recorder := &repositoryPolicyGrantRecorder{t: t, policyReadErr: want}

			got, outcome, err := newEnforcedTestTokenIssuer(t, &signerStub{}, stub).IssueInstallationAccessToken(t.Context(), recorder)

			if !errors.Is(err, want) {
				t.Errorf("IssueInstallationAccessToken error = %v, want %v", err, want)
			}
			if !reflect.ValueOf(got).IsZero() {
				t.Error("IssueInstallationAccessToken returned non-zero ReturnableIssuanceResult with error")
			}
			// ポリシー読み取りトークンの発行 API を呼ばないため、その失効の結果を持たない
			if !outcome.IsZero() {
				t.Errorf("policy read revocation outcome = %v, want the zero value", outcome)
			}
			assertNoRequests(t, stub, issuancePath, repositoryPolicyPath, revocationPath)
			if len(recorder.grantInstallations) != 0 {
				t.Errorf("GrantWithRepositoryPolicies was called %d times, want 0", len(recorder.grantInstallations))
			}
		})
	}
}

// 取得の失敗は発行許可を確定せずに取得の失敗の種別で終わり、ポリシー読み取りトークンの失効の結果を返す。
//
//trace:checks AC-QUALITY-001 AC-ISSUANCE-011
func TestTokenIssuer_IssueInstallationAccessToken_WithRepositoryPolicies_FetchFailureIssuesNoTokenGrant(t *testing.T) {
	handlers := enforcedRoutes(t)
	handlers[repositoryPolicyPath] = respond(http.StatusInternalServerError, requestID, responseMarker)
	stub := newGitHubStub(t, handlers)
	recorder := &repositoryPolicyGrantRecorder{t: t}

	got, outcome, err := newEnforcedTestTokenIssuer(t, &signerStub{}, stub).IssueInstallationAccessToken(t.Context(), recorder)

	if !errors.Is(err, github.ErrTransientFailure) {
		t.Fatalf("IssueInstallationAccessToken error = %v, want %v", err, github.ErrTransientFailure)
	}
	if _, ok := errors.AsType[*VerificationFailure](err); ok {
		t.Errorf("IssueInstallationAccessToken error = %v, want no *VerificationFailure", err)
	}
	if !reflect.ValueOf(got).IsZero() {
		t.Error("IssueInstallationAccessToken returned non-zero ReturnableIssuanceResult with error")
	}
	if outcome != revocation.Succeeded {
		t.Errorf("policy read revocation outcome = %v, want %v", outcome, revocation.Succeeded)
	}
	if len(recorder.grantInstallations) != 0 {
		t.Errorf("GrantWithRepositoryPolicies was called %d times, want 0", len(recorder.grantInstallations))
	}
	// 発行はポリシー読み取りトークンの 1 回だけである
	if n := len(stub.received(issuancePath)); n != 1 {
		t.Errorf("issuance received %d requests, want 1", n)
	}
	assertHidesValues(t, err)
}

//trace:checks AC-AUTHZ-014
func TestTokenIssuer_IssueInstallationAccessToken_WithRepositoryPolicies_TokenGrantFailureIssuesNoToken(t *testing.T) {
	want := fmt.Errorf("%w: repositories[1]: condition", authz.ErrAuthorizationDenied)
	stub := newGitHubStub(t, enforcedRoutes(t))

	got, outcome, err := newEnforcedTestTokenIssuer(t, &signerStub{}, stub).IssueInstallationAccessToken(t.Context(), &repositoryPolicyGrantRecorder{t: t, grantErr: want})

	if !errors.Is(err, want) {
		t.Errorf("IssueInstallationAccessToken error = %v, want %v", err, want)
	}
	if !reflect.ValueOf(got).IsZero() {
		t.Error("IssueInstallationAccessToken returned non-zero ReturnableIssuanceResult with error")
	}
	if outcome != revocation.Succeeded {
		t.Errorf("policy read revocation outcome = %v, want %v", outcome, revocation.Succeeded)
	}
	// 発行はポリシー読み取りトークンの 1 回だけである
	if n := len(stub.received(issuancePath)); n != 1 {
		t.Errorf("issuance received %d requests, want 1", n)
	}
}

// ポリシー読み取りトークンの照合失敗は、利用者に返すトークンの照合失敗と同じ種別で、利用者に返すトークンを発行しないため、その失効を試みなかったことを持つ。
//
//trace:checks AC-ISSUANCE-011
func TestTokenIssuer_IssueInstallationAccessToken_WithRepositoryPolicies_PolicyReadTokenVerificationFailure(t *testing.T) {
	response := policyReadIssuanceWithMarker()
	response["single_file_paths"] = []any{"README.md"}
	handlers := enforcedRoutes(t)
	handlers[issuancePath] = issuanceByKind(respond(http.StatusCreated, requestID, marshal(t, response)), respond(http.StatusCreated, requestID, marshal(t, recordedIssuance())))
	stub := newGitHubStub(t, handlers)
	recorder := &repositoryPolicyGrantRecorder{t: t}

	got, outcome, err := newEnforcedTestTokenIssuer(t, &signerStub{}, stub).IssueInstallationAccessToken(t.Context(), recorder)

	if !errors.Is(err, ErrVerificationFailure) {
		t.Fatalf("IssueInstallationAccessToken error = %v, want %v", err, ErrVerificationFailure)
	}
	failure, ok := errors.AsType[*VerificationFailure](err)
	if !ok {
		t.Fatalf("IssueInstallationAccessToken error = %v, want *VerificationFailure", err)
	}
	if failure.RevocationOutcome() != revocation.NotAttempted {
		t.Errorf("RevocationOutcome() = %v, want %v", failure.RevocationOutcome(), revocation.NotAttempted)
	}
	if !strings.HasPrefix(err.Error(), "issue policy read token: ") {
		t.Errorf("error message %q does not name the stage", err)
	}
	if !reflect.ValueOf(got).IsZero() {
		t.Error("IssueInstallationAccessToken returned non-zero ReturnableIssuanceResult with error")
	}
	if outcome != revocation.Succeeded {
		t.Errorf("policy read revocation outcome = %v, want %v", outcome, revocation.Succeeded)
	}
	assertNoRequests(t, stub, repositoryPolicyPath)
	if len(recorder.grantInstallations) != 0 {
		t.Errorf("GrantWithRepositoryPolicies was called %d times, want 0", len(recorder.grantInstallations))
	}
	if got := stub.received(revocationPath); !reflect.DeepEqual(got, []string{"Bearer " + policyReadTokenMarker}) {
		t.Errorf("revocation received Authorization %q, want [%q]", got, "Bearer "+policyReadTokenMarker)
	}
	assertHidesValues(t, err)
}

// 利用者に返すトークンの実効対象の Repository ID が読んだ対象と違えば、トークンを返さずに失効を試みる。
//
//trace:checks AC-ISSUANCE-012
func TestTokenIssuer_IssueInstallationAccessToken_WithRepositoryPolicies_RevokesTokenWithRepositoryIDDifferentFromReadTarget(t *testing.T) {
	tests := []struct {
		name       string
		revocation http.HandlerFunc
		want       revocation.Outcome
	}{
		{name: "revocation with 204 is read as succeeded", revocation: respond(http.StatusNoContent, revocationRequestID, ""), want: revocation.Succeeded},
		{name: "revocation with 500 is read as failed", revocation: respond(http.StatusInternalServerError, revocationRequestID, responseMarker), want: revocation.Failed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// recreated は、リポジトリポリシーを読んだ後に repo-b を削除して作り直した場合の発行応答である
			recreated := recordedIssuance()
			recreated["repositories"] = []any{
				effectiveRepository(repositoryIDA, allowedOwnerID, allowedOwnerName+"/example-repo"),
				effectiveRepository(1234567899, allowedOwnerID, allowedOwnerName+"/repo-b"),
			}
			handlers := enforcedRoutes(t)
			handlers[issuancePath] = issuanceByKind(respond(http.StatusCreated, requestID, marshal(t, policyReadIssuanceWithMarker())), respond(http.StatusCreated, requestID, marshal(t, recreated)))
			handlers[revocationPath] = func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") == "Bearer "+tokenMarker {
					tt.revocation(w, r)
					return
				}
				respond(http.StatusNoContent, revocationRequestID, "")(w, r)
			}
			stub := newGitHubStub(t, handlers)

			got, outcome, err := newEnforcedTestTokenIssuer(t, &signerStub{}, stub).IssueInstallationAccessToken(t.Context(), &repositoryPolicyGrantRecorder{t: t})

			failure, ok := errors.AsType[*VerificationFailure](err)
			if !ok {
				t.Fatalf("IssueInstallationAccessToken error = %v, want *VerificationFailure", err)
			}
			if failure.RevocationOutcome() != tt.want {
				t.Errorf("RevocationOutcome() = %v, want %v", failure.RevocationOutcome(), tt.want)
			}
			if !strings.Contains(err.Error(), "repositories[1] id does not match the repository whose repository policy was read") {
				t.Errorf("error message %q does not contain the unmet condition", err)
			}
			if !reflect.ValueOf(got).IsZero() {
				t.Error("IssueInstallationAccessToken returned non-zero ReturnableIssuanceResult with error")
			}
			if outcome != revocation.Succeeded {
				t.Errorf("policy read revocation outcome = %v, want %v", outcome, revocation.Succeeded)
			}
			want := []string{"Bearer " + policyReadTokenMarker, "Bearer " + tokenMarker}
			if got := stub.received(revocationPath); !reflect.DeepEqual(got, want) {
				t.Errorf("revocation received Authorization %q, want %q", got, want)
			}
			assertHidesValues(t, err)
		})
	}
}

// 要求の ctx は包まずに取得の段階へ渡すため、ポリシー読み取りトークンの失効も要求の期限の 1 秒前で打ち切る。
func TestTokenIssuer_IssueInstallationAccessToken_WithRepositoryPolicies_PolicyReadRevocationEndsOneSecondBeforeRequestDeadline(t *testing.T) {
	stub := newGitHubStub(t, enforcedRoutes(t))
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	requestDeadline, _ := ctx.Deadline()

	if _, _, err := newEnforcedTestTokenIssuer(t, &signerStub{}, stub).IssueInstallationAccessToken(ctx, &repositoryPolicyGrantRecorder{t: t}); err != nil {
		t.Fatalf("IssueInstallationAccessToken returned error: %v", err)
	}

	got, ok := stub.revocationContext(t).Deadline()
	if want := requestDeadline.Add(-time.Second); !ok || !got.Equal(want) {
		t.Errorf("policy read revocation request deadline = %v (set %v), want %v", got, ok, want)
	}
}

// ポリシー読み取りトークンの失効の失敗は、発行の結果を変えず、失効の結果だけが失敗になる。
//
//trace:checks AC-ISSUANCE-011
func TestTokenIssuer_IssueInstallationAccessToken_WithRepositoryPolicies_PolicyReadRevocationFailureReturnsToken(t *testing.T) {
	handlers := enforcedRoutes(t)
	handlers[revocationPath] = respond(http.StatusInternalServerError, revocationRequestID, responseMarker)

	got, outcome, err := newEnforcedTestTokenIssuer(t, &signerStub{}, newGitHubStub(t, handlers)).IssueInstallationAccessToken(t.Context(), &repositoryPolicyGrantRecorder{t: t})

	if err != nil {
		t.Fatalf("IssueInstallationAccessToken returned error: %v", err)
	}
	// トークン文字列は Lambda Handler だけが取り出せるため、ポリシー読み取りトークンと異なる実効権限で見分ける
	if got, want := maps.Collect(got.EffectivePermissions().Entries()), map[string]string{"contents": "read", "metadata": "read"}; !maps.Equal(got, want) {
		t.Errorf("effective permissions = %v, want %v of the token issued with the token grant", got, want)
	}
	if outcome != revocation.Failed {
		t.Errorf("policy read revocation outcome = %v, want %v", outcome, revocation.Failed)
	}
}

// ポリシー読み取りトークンの発行 API の失敗はトークン文字列を持たないため、失効を試みなかったことを返す。
//
//trace:checks AC-ISSUANCE-011
func TestTokenIssuer_IssueInstallationAccessToken_WithRepositoryPolicies_PolicyReadIssuanceRejected(t *testing.T) {
	handlers := enforcedRoutes(t)
	handlers[issuancePath] = respond(http.StatusUnprocessableEntity, requestID, `{"token":"`+policyReadTokenMarker+`","message":"`+responseMarker+`"}`)
	stub := newGitHubStub(t, handlers)
	recorder := &repositoryPolicyGrantRecorder{t: t}

	_, outcome, err := newEnforcedTestTokenIssuer(t, &signerStub{}, stub).IssueInstallationAccessToken(t.Context(), recorder)

	if !errors.Is(err, github.ErrIssuanceRejected) {
		t.Fatalf("IssueInstallationAccessToken error = %v, want %v", err, github.ErrIssuanceRejected)
	}
	if _, ok := errors.AsType[*VerificationFailure](err); ok {
		t.Errorf("IssueInstallationAccessToken error = %v, want no *VerificationFailure", err)
	}
	if outcome != revocation.NotAttempted {
		t.Errorf("policy read revocation outcome = %v, want %v", outcome, revocation.NotAttempted)
	}
	if n := len(stub.received(issuancePath)); n != 1 {
		t.Errorf("issuance received %d requests, want 1", n)
	}
	assertNoRequests(t, stub, repositoryPolicyPath, revocationPath)
	if len(recorder.grantInstallations) != 0 {
		t.Errorf("GrantWithRepositoryPolicies was called %d times, want 0", len(recorder.grantInstallations))
	}
	assertHidesValues(t, err)
}

// 利用者に返すトークンの発行 API の失敗でも、すでに試みたポリシー読み取りトークンの失効の結果を返す。
//
//trace:checks AC-ISSUANCE-011
func TestTokenIssuer_IssueInstallationAccessToken_WithRepositoryPolicies_TokenIssuanceRejected(t *testing.T) {
	handlers := enforcedRoutes(t)
	handlers[issuancePath] = issuanceByKind(
		respond(http.StatusCreated, requestID, marshal(t, policyReadIssuanceWithMarker())),
		respond(http.StatusUnprocessableEntity, requestID, `{"token":"`+tokenMarker+`","message":"`+responseMarker+`"}`),
	)
	stub := newGitHubStub(t, handlers)

	_, outcome, err := newEnforcedTestTokenIssuer(t, &signerStub{}, stub).IssueInstallationAccessToken(t.Context(), &repositoryPolicyGrantRecorder{t: t})

	if !errors.Is(err, github.ErrIssuanceRejected) {
		t.Fatalf("IssueInstallationAccessToken error = %v, want %v", err, github.ErrIssuanceRejected)
	}
	if !strings.HasPrefix(err.Error(), "issue: ") {
		t.Errorf("error message %q does not name the stage", err)
	}
	if outcome != revocation.Succeeded {
		t.Errorf("policy read revocation outcome = %v, want %v", outcome, revocation.Succeeded)
	}
	// 失効はポリシー読み取りトークンの 1 回だけである
	if got := stub.received(revocationPath); !reflect.DeepEqual(got, []string{"Bearer " + policyReadTokenMarker}) {
		t.Errorf("revocation received Authorization %q, want [%q]", got, "Bearer "+policyReadTokenMarker)
	}
	assertHidesValues(t, err)
}

func (g *repositoryPolicyGrantRecorder) GrantPolicyRead(installation authz.Installation) (authz.PolicyReadGrant, error) {
	g.policyReadInstallations = append(g.policyReadInstallations, installation)
	if g.policyReadErr != nil {
		return authz.PolicyReadGrant{}, g.policyReadErr
	}
	inputs := newGrantInputs(g.t, contentsRead)
	return authz.GrantPolicyRead(inputs.caller, inputs.request, allowedOwner(g.t), inputs.issuablePermissions, noProtectedRepositories(g.t), authz.RepositoryPolicyEnforced, installation)
}

func (g *repositoryPolicyGrantRecorder) GrantWithRepositoryPolicies(installation authz.Installation, fetchResults []repositorypolicy.FetchResult) (authz.TokenGrant, error) {
	g.grantInstallations = append(g.grantInstallations, installation)
	g.fetchResults = append(g.fetchResults, fetchResults)
	if g.grantErr != nil {
		return authz.TokenGrant{}, g.grantErr
	}
	inputs := newGrantInputs(g.t, contentsRead)
	return authz.GrantWithRepositoryPolicies(inputs.caller, inputs.request, allowedOwner(g.t), inputs.issuablePermissions, noProtectedRepositories(g.t), authz.RepositoryPolicyEnforced, installation, fetchResults)
}

func (g *repositoryPolicyGrantRecorder) Grant(authz.Installation) (authz.TokenGrant, error) {
	g.t.Error("Grant was called under repository policy enforcement")
	return authz.TokenGrant{}, authz.ErrAuthorizationDenied
}

// enforcedRoutes は、リポジトリポリシーを適用するデプロイの発行処理が成功する経路である。
// 発行の経路は、ポリシー読み取りトークンと利用者に返すトークンの発行応答を出し分け、取得の経路は、Caller を許す対象ごとの文書を返す。
func enforcedRoutes(t *testing.T) map[string]http.HandlerFunc {
	t.Helper()
	return map[string]http.HandlerFunc{
		issuancePath: issuanceByKind(respond(http.StatusCreated, requestID, marshal(t, policyReadIssuanceWithMarker())), respond(http.StatusCreated, requestID, marshal(t, recordedIssuance()))),
		repositoryPolicyPath: func(w http.ResponseWriter, r *http.Request) {
			respond(http.StatusOK, requestID, repositoryPolicyFileOf(t, allowingDocumentOf(r.PathValue("repo"))))(w, r)
		},
	}
}

// policyReadIssuanceWithMarker は、トークンを利用者に返すトークンと見分けられる目印にした、ポリシー読み取りトークンの発行応答の形である。
func policyReadIssuanceWithMarker() map[string]any {
	response := recordedPolicyReadIssuance()
	response["token"] = policyReadTokenMarker
	return response
}

// allowingDocumentOf は、Caller（example-repo）の contents:read を許す、どの対象の文書かを見分けられるリポジトリポリシーの文書である。
func allowingDocumentOf(repo string) string {
	return fmt.Sprintf("callers:\n  - repository_id: \"%d\"\n    repository_name: example-repo\n    permissions:\n      contents: read\n# %s %s\n", repositoryIDA, documentMarker, repo)
}
