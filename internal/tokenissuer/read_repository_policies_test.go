package tokenissuer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/github"
	"github.com/tmknom/gates/internal/nonzero"
	"github.com/tmknom/gates/internal/repositorypolicy"
	"github.com/tmknom/gates/internal/revocation"
)

// ポリシー読み取りトークンを発行し、照合し、対象を順に取得し、失効させる。
// 各段階の要求の資格情報と、取得の結果と Repository ID の対応を確かめる。
//
//trace:checks AC-ISSUANCE-011
func TestReadRepositoryPolicies_ReadsAllTargets(t *testing.T) {
	stub := newGitHubStub(t, policyReadIssuance(t))

	got, outcome, err := readPoliciesFrom(t.Context(), t, stub)

	if err != nil {
		t.Fatalf("readRepositoryPolicies returned error: %v", err)
	}
	if outcome != revocation.Succeeded {
		t.Errorf("outcome = %v, want %v", outcome, revocation.Succeeded)
	}
	// 要求の対象の順に、要求の表記で、照合済みの実効対象の Repository ID を持つ
	want := []repositoryPolicyReading{
		{target: policyReadTarget{fullName: mustParseFullName(t, allowedOwnerName+"/repo-b"), repositoryID: mustRepositoryID(t, repositoryIDB)}, fetchResult: repositorypolicy.Document(documentOf("repo-b"))},
		{target: policyReadTarget{fullName: mustParseFullName(t, allowedOwnerName+"/Example-Repo"), repositoryID: mustRepositoryID(t, repositoryIDA)}, fetchResult: repositorypolicy.Document(documentOf("Example-Repo"))},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("readings = %+v, want %+v", got, want)
	}
	wantAuthorizations := map[string][]string{
		issuancePath:         {"Bearer " + appJWTMarker},
		repositoryPolicyPath: {"Bearer " + tokenMarker, "Bearer " + tokenMarker},
		revocationPath:       {"Bearer " + tokenMarker},
	}
	for pattern, want := range wantAuthorizations {
		if got := stub.received(pattern); !reflect.DeepEqual(got, want) {
			t.Errorf("%s received Authorization %q, want %q", pattern, got, want)
		}
	}
}

// リポジトリポリシーがないことは取得の失敗ではなく、残りの対象の取得を続ける。
func TestReadRepositoryPolicies_ContinuesAfterNotFound(t *testing.T) {
	handlers := policyReadIssuance(t)
	handlers[repositoryPolicyPath] = func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("repo") == "repo-b" {
			respond(http.StatusNotFound, requestID, responseMarker)(w, r)
			return
		}
		respond(http.StatusOK, requestID, repositoryPolicyFile(t, r.PathValue("repo")))(w, r)
	}

	got, _, err := readPoliciesFrom(t.Context(), t, newGitHubStub(t, handlers))

	if err != nil {
		t.Fatalf("readRepositoryPolicies returned error: %v", err)
	}
	fetchResults := []repositorypolicy.FetchResult{got[0].fetchResult, got[1].fetchResult}
	if want := []repositorypolicy.FetchResult{repositorypolicy.NotFound(), repositorypolicy.Document(documentOf("Example-Repo"))}; !reflect.DeepEqual(fetchResults, want) {
		t.Errorf("fetch results = %+v, want %+v", fetchResults, want)
	}
}

// 発行 API の失敗はトークン文字列を持たないため、失効を試みず、取得もしない。
//
//trace:checks AC-QUALITY-001
func TestReadRepositoryPolicies_IssuanceFailureDoesNotRevoke(t *testing.T) {
	tests := []struct {
		status int
		want   error
	}{
		{status: http.StatusUnprocessableEntity, want: github.ErrIssuanceRejected},
		{status: http.StatusInternalServerError, want: github.ErrTransientFailure},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("status %d returns %v", tt.status, tt.want), func(t *testing.T) {
			body := `{"token":"` + tokenMarker + `","message":"` + responseMarker + `"}`
			stub := newGitHubStub(t, map[string]http.HandlerFunc{issuancePath: respond(tt.status, requestID, body)})

			got, outcome, err := readPoliciesFrom(t.Context(), t, stub)

			if !errors.Is(err, tt.want) {
				t.Fatalf("readRepositoryPolicies error = %v, want %v", err, tt.want)
			}
			if !strings.HasPrefix(err.Error(), "issue policy read token: ") {
				t.Errorf("error message %q does not name the stage", err)
			}
			if outcome != revocation.NotAttempted {
				t.Errorf("outcome = %v, want %v", outcome, revocation.NotAttempted)
			}
			if got != nil {
				t.Errorf("readings = %+v, want nil", got)
			}
			assertNoRequests(t, stub, repositoryPolicyPath, revocationPath)
			assertHidesValues(t, err)
		})
	}
}

// 照合を通らないポリシー読み取りトークンでは取得せず、失効を試みる。
// 照合失敗は利用者に返すトークンの照合失敗と同じ種別だが、その失効の結果（*VerificationFailure）は持たない。
//
//trace:checks AC-ISSUANCE-011
func TestReadRepositoryPolicies_VerificationFailureRevokesWithoutFetching(t *testing.T) {
	response := recordedPolicyReadIssuance()
	response["single_file_paths"] = []any{"README.md"}
	stub := newGitHubStub(t, map[string]http.HandlerFunc{issuancePath: respond(http.StatusCreated, requestID, marshal(t, response))})

	got, outcome, err := readPoliciesFrom(t.Context(), t, stub)

	if !errors.Is(err, ErrVerificationFailure) {
		t.Fatalf("readRepositoryPolicies error = %v, want %v", err, ErrVerificationFailure)
	}
	if _, ok := errors.AsType[*VerificationFailure](err); ok {
		t.Errorf("readRepositoryPolicies error = %v, want no *VerificationFailure", err)
	}
	if !strings.HasPrefix(err.Error(), "issue policy read token: ") {
		t.Errorf("error message %q does not name the stage", err)
	}
	if outcome != revocation.Succeeded {
		t.Errorf("outcome = %v, want %v", outcome, revocation.Succeeded)
	}
	if got != nil {
		t.Errorf("readings = %+v, want nil", got)
	}
	assertNoRequests(t, stub, repositoryPolicyPath)
	if got := stub.received(revocationPath); !reflect.DeepEqual(got, []string{"Bearer " + tokenMarker}) {
		t.Errorf("revocation received Authorization %q, want [%q]", got, "Bearer "+tokenMarker)
	}
	assertHidesValues(t, err)
}

func TestReadRepositoryPolicies_UnreadableTokenIsNotRevoked(t *testing.T) {
	stub := newGitHubStub(t, map[string]http.HandlerFunc{issuancePath: respond(http.StatusCreated, requestID, "not json "+responseMarker)})

	_, outcome, err := readPoliciesFrom(t.Context(), t, stub)

	if !errors.Is(err, ErrVerificationFailure) {
		t.Fatalf("readRepositoryPolicies error = %v, want %v", err, ErrVerificationFailure)
	}
	if outcome != revocation.NotAttempted {
		t.Errorf("outcome = %v, want %v", outcome, revocation.NotAttempted)
	}
	assertNoRequests(t, stub, repositoryPolicyPath, revocationPath)
}

// 取得の失敗は、残りの対象を取得せずに失効を試みてから、取得の失敗の種別で返す。
//
//trace:checks AC-QUALITY-001 AC-ISSUANCE-011
func TestReadRepositoryPolicies_FetchFailureStopsAndRevokes(t *testing.T) {
	tests := []struct {
		name     string
		response http.HandlerFunc
		want     error
	}{
		{name: "status 500 returns ErrTransientFailure", response: respond(http.StatusInternalServerError, requestID, responseMarker), want: github.ErrTransientFailure},
		{name: "status 403 without rate limit returns ErrInternalFailure", response: respond(http.StatusForbidden, requestID, responseMarker), want: github.ErrInternalFailure},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlers := policyReadIssuance(t)
			handlers[repositoryPolicyPath] = tt.response
			stub := newGitHubStub(t, handlers)

			got, outcome, err := readPoliciesFrom(t.Context(), t, stub)

			if !errors.Is(err, tt.want) {
				t.Fatalf("readRepositoryPolicies error = %v, want %v", err, tt.want)
			}
			if want := "fetch repository policy: repositories[0]: "; !strings.HasPrefix(err.Error(), want) {
				t.Errorf("error message %q does not start with %q", err, want)
			}
			if outcome != revocation.Succeeded {
				t.Errorf("outcome = %v, want %v", outcome, revocation.Succeeded)
			}
			if got != nil {
				t.Errorf("readings = %+v, want nil", got)
			}
			if n := len(stub.received(repositoryPolicyPath)); n != 1 {
				t.Errorf("repository policy fetch received %d requests, want 1", n)
			}
			if n := len(stub.received(revocationPath)); n != 1 {
				t.Errorf("revocation received %d requests, want 1", n)
			}
			assertHidesValues(t, err)
		})
	}
}

// 失効の失敗は取得の結果を変えず、失効の結果だけが失敗になる。段階が失敗で終わるときは、失効の失敗の理由を文言に添える。
func TestReadRepositoryPolicies_RevocationFailure(t *testing.T) {
	t.Run("after all targets are read the readings are returned with failed", func(t *testing.T) {
		handlers := policyReadIssuance(t)
		handlers[revocationPath] = respond(http.StatusInternalServerError, revocationRequestID, responseMarker)

		got, outcome, err := readPoliciesFrom(t.Context(), t, newGitHubStub(t, handlers))

		if err != nil {
			t.Fatalf("readRepositoryPolicies returned error: %v", err)
		}
		if outcome != revocation.Failed {
			t.Errorf("outcome = %v, want %v", outcome, revocation.Failed)
		}
		if len(got) != 2 {
			t.Errorf("readings = %d, want 2", len(got))
		}
	})
	t.Run("after a fetch failure the reason is added to the fetch failure", func(t *testing.T) {
		handlers := policyReadIssuance(t)
		handlers[repositoryPolicyPath] = respond(http.StatusInternalServerError, requestID, responseMarker)
		handlers[revocationPath] = respond(http.StatusInternalServerError, revocationRequestID, responseMarker)

		_, outcome, err := readPoliciesFrom(t.Context(), t, newGitHubStub(t, handlers))

		if !errors.Is(err, github.ErrTransientFailure) {
			t.Fatalf("readRepositoryPolicies error = %v, want %v", err, github.ErrTransientFailure)
		}
		if outcome != revocation.Failed {
			t.Errorf("outcome = %v, want %v", outcome, revocation.Failed)
		}
		if !strings.Contains(err.Error(), "revocation returned status 500") || !strings.Contains(err.Error(), revocationRequestID) {
			t.Errorf("error message %q does not contain the revocation reason", err)
		}
		assertHidesValues(t, err)
	})
}

// 要求の ctx が取得の途中で中断されても、失効は要求の ctx の中断を引き継がずに試みる。
func TestReadRepositoryPolicies_RevokesAfterRequestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	handlers := policyReadIssuance(t)
	handlers[repositoryPolicyPath] = func(_ http.ResponseWriter, r *http.Request) {
		cancel()
		<-r.Context().Done()
	}
	stub := newGitHubStub(t, handlers)

	_, outcome, err := readPoliciesFrom(ctx, t, stub)

	if !errors.Is(err, github.ErrTransientFailure) {
		t.Fatalf("readRepositoryPolicies error = %v, want %v", err, github.ErrTransientFailure)
	}
	if outcome != revocation.Succeeded {
		t.Errorf("outcome = %v, want %v", outcome, revocation.Succeeded)
	}
	if n := len(stub.received(revocationPath)); n != 1 {
		t.Errorf("revocation received %d requests, want 1", n)
	}
}

// 失効は、要求の ctx の期限から応答を書くための 1 秒を除いた時刻で打ち切る。
func TestReadRepositoryPolicies_RevocationEndsOneSecondBeforeRequestDeadline(t *testing.T) {
	stub := newGitHubStub(t, policyReadIssuance(t))
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	requestDeadline, _ := ctx.Deadline()

	if _, _, err := readPoliciesFrom(ctx, t, stub); err != nil {
		t.Fatalf("readRepositoryPolicies returned error: %v", err)
	}

	got, ok := stub.revocationContext(t).Deadline()
	if want := requestDeadline.Add(-time.Second); !ok || !got.Equal(want) {
		t.Errorf("revocation request deadline = %v (set %v), want %v", got, ok, want)
	}
}

// panic で段階を抜ける場合も、抜ける前に失効を試みる。
//
//trace:checks AC-ISSUANCE-011
func TestReadRepositoryPolicies_RevokesWhenFetchPanics(t *testing.T) {
	stub := newGitHubStub(t, policyReadIssuance(t))
	apiHost, err := url.Parse(stub.server.URL)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	base := stub.server.Client().Transport
	transport := roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodGet && strings.Contains(request.URL.Path, "/contents/") {
			// コンポーネントが panic するのはゼロ値を前提の処理に渡した場合だけのため（Lambda Design の「Failure」）、その経路で panic させる
			nonzero.Require(authz.PolicyReadGrant{})
		}
		return base.RoundTrip(request)
	})
	tokenIssuer := NewTokenIssuer(&signerStub{}, github.NewClient(github.WithAPIHost(apiHost), github.WithTransport(transport)), allowedOwner(t), noProtectedRepositories(t), authz.RepositoryPolicyEnforced, func() time.Time { return now })
	defer func() {
		if recover() == nil {
			t.Error("readRepositoryPolicies did not panic")
		}
		if n := len(stub.received(revocationPath)); n != 1 {
			t.Errorf("revocation received %d requests, want 1", n)
		}
	}()

	_, _, _ = tokenIssuer.readRepositoryPolicies(t.Context(), github.NewAppJWT(appJWTMarker), policyReadGrant(t))
}

// 取得の合計の予算は最初の取得から 15 秒で、使い切ったら残りの対象を取得せずに一時的な障害にする。
// 各取得は 4 秒かかるため、4 件目は 3 秒で打ち切られ、5 件目は送らない。
func TestReadRepositoryPolicies_StopsWhenFetchBudgetIsExhausted(t *testing.T) {
	names := []string{"repo-0", "repo-1", "repo-2", "repo-3", "repo-4"}
	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		var fetchDeadlines []time.Time
		var revoked int
		transport := roundTripperFunc(func(request *http.Request) (*http.Response, error) {
			switch request.Method {
			case http.MethodPost:
				return fakeResponse(http.StatusCreated, marshal(t, policyReadIssuanceOf(names))), nil
			case http.MethodDelete:
				revoked++
				return fakeResponse(http.StatusNoContent, ""), nil
			}
			deadline, _ := request.Context().Deadline()
			fetchDeadlines = append(fetchDeadlines, deadline)
			select {
			case <-time.After(4 * time.Second):
				return fakeResponse(http.StatusOK, repositoryPolicyFile(t, "repo")), nil
			case <-request.Context().Done():
				return nil, request.Context().Err()
			}
		})
		repositories := make([]string, 0, len(names))
		for _, name := range names {
			repositories = append(repositories, allowedOwnerName+"/"+name)
		}
		tokenIssuer := NewTokenIssuer(&signerStub{}, github.NewClient(github.WithTransport(transport)), allowedOwner(t), noProtectedRepositories(t), authz.RepositoryPolicyEnforced, func() time.Time { return now })

		_, _, err := tokenIssuer.readRepositoryPolicies(t.Context(), github.NewAppJWT(appJWTMarker), policyReadGrantOf(t, repositories))

		if !errors.Is(err, github.ErrTransientFailure) {
			t.Fatalf("readRepositoryPolicies error = %v, want %v", err, github.ErrTransientFailure)
		}
		if want := "fetch repository policy: repositories[3]: "; !strings.HasPrefix(err.Error(), want) {
			t.Errorf("error message %q does not start with %q", err, want)
		}
		want := []time.Time{start.Add(5 * time.Second), start.Add(9 * time.Second), start.Add(13 * time.Second), start.Add(15 * time.Second)}
		if !reflect.DeepEqual(fetchDeadlines, want) {
			t.Errorf("fetch deadlines = %v, want %v", fetchDeadlines, want)
		}
		if revoked != 1 {
			t.Errorf("revocation requests = %d, want 1", revoked)
		}
	})
}

// 段階の戻り値は資格情報を持たず、エラー文言は文書の内容を含まない。
//
//trace:checks AC-QUALITY-002 AC-ISSUANCE-011
func TestReadRepositoryPolicies_DoesNotExposeTokenOrDocument(t *testing.T) {
	got, _, _ := readPoliciesFrom(t.Context(), t, newGitHubStub(t, policyReadIssuance(t)))
	if output := fmt.Sprintf("%+v %#v", got, got); strings.Contains(output, tokenMarker) {
		t.Errorf("readings output %q contains the policy read token", output)
	}

	handlers := policyReadIssuance(t)
	handlers[repositoryPolicyPath] = func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("repo") == "repo-b" {
			respond(http.StatusOK, requestID, repositoryPolicyFile(t, "repo-b"))(w, r)
			return
		}
		respond(http.StatusInternalServerError, requestID, documentMarker)(w, r)
	}
	_, _, err := readPoliciesFrom(t.Context(), t, newGitHubStub(t, handlers))
	if err == nil {
		t.Fatal("readRepositoryPolicies returned no error")
	}
	for _, value := range []string{tokenMarker, documentMarker, appJWTMarker} {
		if strings.Contains(err.Error(), value) {
			t.Errorf("error message %q contains %q", err, value)
		}
	}
}

// readPoliciesFrom は、stub に向けた Token Issuer で、requestedRepositories を対象とする取得の段階を呼ぶ。
func readPoliciesFrom(ctx context.Context, t *testing.T, stub *gitHubStub) ([]repositoryPolicyReading, revocation.Outcome, error) {
	t.Helper()
	return newEnforcedTestTokenIssuer(t, &signerStub{}, stub).readRepositoryPolicies(ctx, github.NewAppJWT(appJWTMarker), policyReadGrant(t))
}

// policyReadIssuance は、発行の経路がポリシー読み取りトークンの発行応答を返すハンドラである。
func policyReadIssuance(t *testing.T) map[string]http.HandlerFunc {
	t.Helper()
	return map[string]http.HandlerFunc{issuancePath: respond(http.StatusCreated, requestID, marshal(t, recordedPolicyReadIssuance()))}
}

// policyReadIssuanceOf は、names のリポジトリを実効対象とする、照合を通るポリシー読み取りトークンの発行応答を返す。
func policyReadIssuanceOf(names []string) map[string]any {
	response := recordedPolicyReadIssuance()
	repositories := make([]any, 0, len(names))
	for i, name := range names {
		repositories = append(repositories, effectiveRepository(int64(2000000000+i), allowedOwnerID, allowedOwnerName+"/"+name))
	}
	response["repositories"] = repositories
	return response
}

func fakeResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
}
