package github_test

import (
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/caller"
	"github.com/tmknom/gates/internal/github"
	"github.com/tmknom/gates/internal/permission"
	"github.com/tmknom/gates/internal/request"
)

const (
	// tokenMarker は、トークンがエラー文言に漏れていないことを確かめる目印である。
	tokenMarker = "ghs_token-marker" //nolint:gosec // 資格情報ではなく、漏れを探すための目印である

	// issuanceSizeLimit と issuanceTimeout は Lambda Design の GitHub Client と Call Time Budget が定める値である。
	issuanceSizeLimit = 16 * 1024 * 1024
	issuanceTimeout   = 10 * time.Second
)

// requestedRepositories は、発行許可の対象である。表記と並びが要求のまま発行 API に渡ることを確かめるため、大文字を含め、名前の昇順にしない。
var requestedRepositories = []string{allowedOwnerName + "/repo-b", allowedOwnerName + "/Example-Repo"}

// recordedIssuance は、調査記録の発行応答を、ケースごとに書き換えられる形で返す。リポジトリの表現は照合に使うキーと一部のキーだけにする。
func recordedIssuance() map[string]any {
	return map[string]any{
		"token":                tokenMarker,
		"expires_at":           "2026-11-03T12:18:48Z",
		"permissions":          map[string]any{"contents": "read", "metadata": "read"},
		"repository_selection": "selected",
		"repositories": []any{
			map[string]any{
				"id":        1234567890,
				"name":      "example-repo",
				"full_name": "octocat/example-repo",
				"private":   true,
				"owner":     map[string]any{"login": "octocat", "id": recordedAccountID, "type": "User"},
			},
			map[string]any{
				"id":        1234567891,
				"name":      "repo-b",
				"full_name": "octocat/repo-b",
				"private":   true,
				"owner":     map[string]any{"login": "octocat", "id": recordedAccountID, "type": "User"},
			},
		},
	}
}

func respondIssuance(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-GitHub-Request-Id", requestID)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, body)
	}
}

// tokenGrant は、TokenGrant を Grant でしか作れないため、Caller、形式検証を通した発行要求、許可オーナー、Installation をそろえて Grant を通す。
func tokenGrant(t *testing.T) authz.TokenGrant {
	t.Helper()
	c, err := caller.NewCaller(recordedAccountID, 1234567890, 4004, 1, caller.WorkflowTriggerAttributes{})
	if err != nil {
		t.Fatalf("NewCaller returned error: %v", err)
	}
	payload := marshal(t, map[string]any{
		"version":      1,
		"id_token":     "header.payload.signature",
		"repositories": requestedRepositories,
		"permissions":  map[string]string{"contents": "read"},
	})
	_, req, err := request.Validate([]byte(payload))
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	grant, err := authz.Grant(c, req, allowedOwner(t, authz.OwnerTypeUser), newInstallation(t, time.Time{}))
	if err != nil {
		t.Fatalf("Grant returned error: %v", err)
	}
	return grant
}

func issue(t *testing.T, client *github.Client) (github.IssuanceResult, error) {
	t.Helper()
	return client.Issue(t.Context(), github.NewAppJWT(appJWTValue), tokenGrant(t))
}

func issueBody(t *testing.T, body string) github.IssuanceResult {
	t.Helper()
	got, err := issue(t, newAPIStub(t, respondIssuance(body)).client(t))
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}
	return got
}

// readFields は、発行結果の項目ごとに、応答から読み取れたかを返す。
func readFields(result github.IssuanceResult) map[string]bool {
	_, token := result.Token()
	_, expiresAt := result.ExpiresAt()
	_, permissions := result.EffectivePermissions()
	_, selection := result.RepositorySelection()
	_, repositories := result.EffectiveRepositories()
	return map[string]bool{
		"token":                token,
		"expires_at":           expiresAt,
		"permissions":          permissions,
		"repository_selection": selection,
		"repositories":         repositories,
	}
}

// assertReadFields は、unreadable に挙げた項目だけを読み取れなかったことを確かめる。
func assertReadFields(t *testing.T, result github.IssuanceResult, unreadable ...string) {
	t.Helper()
	for field, read := range readFields(result) {
		if want := !slices.Contains(unreadable, field); read != want {
			t.Errorf("field %s read = %t, want %t", field, read, want)
		}
	}
}

var allFields = []string{"token", "expires_at", "permissions", "repository_selection", "repositories"}

//trace:checks AC-09 AC-11
func TestIssue_SendsExplicitRepositoriesAndPermissions(t *testing.T) {
	var body atomic.Pointer[string]
	stub := newAPIStub(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		s := string(b)
		body.Store(&s)
		respondIssuance(marshal(t, recordedIssuance()))(w, r)
	})

	if _, err := issue(t, stub.client(t)); err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	req := stub.request.Load()
	if want := "/app/installations/" + strconv.Itoa(recordedInstallationID) + "/access_tokens"; req.Method != http.MethodPost || req.URL.Path != want {
		t.Errorf("request = %s %s, want POST %s", req.Method, req.URL.Path, want)
	}
	if got := req.Header.Values("Content-Type"); !slices.Equal(got, []string{"application/json"}) {
		t.Errorf("request header Content-Type = %q, want [\"application/json\"]", got)
	}
	var got struct {
		Repositories []string          `json:"repositories"`
		Permissions  map[string]string `json:"permissions"`
	}
	// 未知のキーを拒否し、発行許可にない値を本文に足していないことも確かめる
	if err := json.Unmarshal([]byte(*body.Load()), &got, json.RejectUnknownMembers(true)); err != nil {
		t.Fatalf("request body %q is not the issuance request: %v", *body.Load(), err)
	}
	if want := []string{"repo-b", "Example-Repo"}; !slices.Equal(got.Repositories, want) {
		t.Errorf("request repositories = %q, want %q", got.Repositories, want)
	}
	if want := map[string]string{"contents": "read"}; !maps.Equal(got.Permissions, want) {
		t.Errorf("request permissions = %v, want %v", got.Permissions, want)
	}
}

func TestIssue_SendsRequiredHeaders(t *testing.T) {
	sent := sentIssuanceRequest(t)
	wantHeader := map[string]string{
		"Authorization":        "Bearer " + appJWTValue,
		"Accept":               "application/vnd.github+json",
		"X-GitHub-Api-Version": "2026-03-10",
		"User-Agent":           "gates",
	}
	for name, want := range wantHeader {
		if got := sent.Header.Values(name); len(got) != 1 || got[0] != want {
			t.Errorf("request header %s = %q, want [%q]", name, got, want)
		}
	}
}

// sentIssuanceRequest は、既定の API Host のまま Issue を呼び、外へ送らずに受け取った要求を返す。
func sentIssuanceRequest(t *testing.T) *http.Request {
	t.Helper()
	var sent *http.Request
	transport := roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		sent = request
		return &http.Response{
			StatusCode: http.StatusCreated,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader(marshal(t, recordedIssuance()))),
		}, nil
	})
	if _, err := issue(t, github.NewClient(github.WithTransport(transport))); err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}
	return sent
}

func TestIssue_SendsToGitHubAPIHost(t *testing.T) {
	want := "https://api.github.com/app/installations/" + strconv.Itoa(recordedInstallationID) + "/access_tokens"
	if got := sentIssuanceRequest(t).URL.String(); got != want {
		t.Errorf("request URL = %q, want %q", got, want)
	}
}

func TestIssue_WaitsAtMostIssuanceTimeout(t *testing.T) {
	deadline, ok := sentIssuanceRequest(t).Context().Deadline()
	if !ok {
		t.Fatal("request has no deadline")
	}
	if remaining := time.Until(deadline); remaining > issuanceTimeout || remaining < issuanceTimeout/2 {
		t.Errorf("remaining time until the deadline = %v, want about %v", remaining, issuanceTimeout)
	}
}

func TestIssue_ReadsRecordedResponse(t *testing.T) {
	got := issueBody(t, marshal(t, recordedIssuance()))

	assertReadFields(t, got)
	if got.GitHubRequestID() != requestID {
		t.Errorf("GitHubRequestID() = %q, want %q", got.GitHubRequestID(), requestID)
	}
	if expiresAt, _ := got.ExpiresAt(); !expiresAt.Equal(time.Date(2026, 11, 3, 12, 18, 48, 0, time.UTC)) {
		t.Errorf("ExpiresAt() = %v, want 2026-11-03T12:18:48Z", expiresAt)
	}
	permissions, _ := got.EffectivePermissions()
	if want, _ := permission.NewPermissions(map[string]string{"contents": "read", "metadata": "read"}); !permissions.Equal(want) {
		t.Error("EffectivePermissions() is not {contents:read, metadata:read}")
	}
	if selection, _ := got.RepositorySelection(); selection != github.RepositorySelectionSelected {
		t.Errorf("RepositorySelection() = %q, want %q", selection, github.RepositorySelectionSelected)
	}
	repositories, _ := got.EffectiveRepositories()
	var gotRepositories []string
	for _, r := range repositories {
		gotRepositories = append(gotRepositories, strconv.FormatInt(r.RepositoryID(), 10)+" "+strconv.FormatInt(r.OwnerID(), 10)+" "+r.FullName().String())
	}
	wantRepositories := []string{"1234567890 583231 octocat/example-repo", "1234567891 583231 octocat/repo-b"}
	if !slices.Equal(gotRepositories, wantRepositories) {
		t.Errorf("EffectiveRepositories() = %v, want %v", gotRepositories, wantRepositories)
	}
}

func TestIssue_ReadsEachFieldIndependently(t *testing.T) {
	type change func(body map[string]any)
	set := func(field string, value any) change { return func(body map[string]any) { body[field] = value } }
	drop := func(field string) change { return func(body map[string]any) { delete(body, field) } }
	setRepository := func(key string, value any) change {
		return func(body map[string]any) { body["repositories"].([]any)[1].(map[string]any)[key] = value }
	}
	dropRepository := func(key string) change {
		return func(body map[string]any) { delete(body["repositories"].([]any)[1].(map[string]any), key) }
	}

	tests := []struct {
		name       string
		change     change
		unreadable string
	}{
		{name: "missing token", change: drop("token"), unreadable: "token"},
		{name: "null token", change: set("token", nil), unreadable: "token"},
		{name: "number token", change: set("token", 1), unreadable: "token"},
		{name: "empty token", change: set("token", ""), unreadable: "token"},
		{name: "missing expires_at", change: drop("expires_at"), unreadable: "expires_at"},
		{name: "null expires_at", change: set("expires_at", nil), unreadable: "expires_at"},
		{name: "non time expires_at", change: set("expires_at", "2026-11-03 12:18:48"), unreadable: "expires_at"},
		{name: "missing permissions", change: drop("permissions"), unreadable: "permissions"},
		{name: "null permissions", change: set("permissions", nil), unreadable: "permissions"},
		{name: "array permissions", change: set("permissions", []any{"contents"}), unreadable: "permissions"},
		{name: "empty permissions", change: set("permissions", map[string]any{}), unreadable: "permissions"},
		// Lambda Design の権限名と水準の文字種に合わない権限名
		{name: "malformed permission name", change: set("permissions", map[string]any{"Contents": "read"}), unreadable: "permissions"},
		{name: "missing repository_selection", change: drop("repository_selection"), unreadable: "repository_selection"},
		{name: "null repository_selection", change: set("repository_selection", nil), unreadable: "repository_selection"},
		{name: "number repository_selection", change: set("repository_selection", 1), unreadable: "repository_selection"},
		{name: "missing repositories", change: drop("repositories"), unreadable: "repositories"},
		{name: "null repositories", change: set("repositories", nil), unreadable: "repositories"},
		{name: "object repositories", change: set("repositories", map[string]any{}), unreadable: "repositories"},
		{name: "null repository", change: func(body map[string]any) { body["repositories"].([]any)[1] = nil }, unreadable: "repositories"},
		{name: "missing repository id", change: dropRepository("id"), unreadable: "repositories"},
		{name: "string repository id", change: setRepository("id", "1234567891"), unreadable: "repositories"},
		{name: "missing owner", change: dropRepository("owner"), unreadable: "repositories"},
		{name: "null owner", change: setRepository("owner", nil), unreadable: "repositories"},
		{name: "missing owner id", change: setRepository("owner", map[string]any{"login": "octocat"}), unreadable: "repositories"},
		{name: "missing full_name", change: dropRepository("full_name"), unreadable: "repositories"},
		{name: "full_name with two slashes", change: setRepository("full_name", "octocat/repo/b"), unreadable: "repositories"},
	}
	for _, tt := range tests {
		t.Run(tt.name+" leaves only "+tt.unreadable+" unread", func(t *testing.T) {
			body := recordedIssuance()
			tt.change(body)

			assertReadFields(t, issueBody(t, marshal(t, body)), tt.unreadable)
		})
	}
}

func TestIssue_UninterpretableBodyHasNoToken(t *testing.T) {
	tests := []struct {
		name string
		body func(t *testing.T) string
	}{
		{name: "non JSON body", body: func(*testing.T) string { return `<html>{"token":"` + tokenMarker + `"}` }},
		{name: "array body", body: func(t *testing.T) string { return "[" + marshal(t, recordedIssuance()) + "]" }},
		{name: "duplicate token", body: func(t *testing.T) string {
			return `{"token":"ghs_other",` + strings.TrimPrefix(marshal(t, recordedIssuance()), "{")
		}},
		// encoding/json/v2 は、発行結果に写さないフィールドの中の重複も本文全体の誤りにする
		{name: "duplicate name inside an unread field", body: func(t *testing.T) string {
			return `{"other":{"a":1,"a":2},` + strings.TrimPrefix(marshal(t, recordedIssuance()), "{")
		}},
		{name: "invalid UTF-8 inside an unread field", body: func(t *testing.T) string {
			return `{"other":"\ud800",` + strings.TrimPrefix(marshal(t, recordedIssuance()), "{")
		}},
		{name: "body over the limit", body: func(t *testing.T) string {
			return padded(marshal(t, recordedIssuance()), issuanceSizeLimit+1)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name+" reads no field but keeps the request id", func(t *testing.T) {
			got := issueBody(t, tt.body(t))

			assertReadFields(t, got, allFields...)
			if got.GitHubRequestID() != requestID {
				t.Errorf("GitHubRequestID() = %q, want %q", got.GitHubRequestID(), requestID)
			}
		})
	}
}

func TestIssue_BodyAtTheLimitIsRead(t *testing.T) {
	assertReadFields(t, issueBody(t, padded(marshal(t, recordedIssuance()), issuanceSizeLimit)))
}

// assertIssueFailed は失敗の種別と、発行結果にトークンがなく、エラー文言に GitHub App JWT、トークン、応答本文が入らないことを確かめる。
func assertIssueFailed(t *testing.T, got github.IssuanceResult, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("Issue error = %v, want %v", err, want)
	}
	for _, other := range []error{github.ErrIssuanceRejected, github.ErrTransientFailure, authz.ErrInstallationUnavailable} {
		if !errors.Is(other, want) && errors.Is(err, other) {
			t.Errorf("Issue error = %v, also matches %v", err, other)
		}
	}
	assertReadFields(t, got, allFields...)
	for _, value := range []string{appJWTValue, tokenMarker, responseMarker} {
		if strings.Contains(err.Error(), value) {
			t.Errorf("error message %q contains %q", err.Error(), value)
		}
	}
}

//trace:checks AC-06 AC-10 AC-17 AC-29
func TestIssue_StatusFailure(t *testing.T) {
	tests := []struct {
		name   string
		status int
		header map[string]string
		want   error
	}{
		{name: "status 422", status: 422, want: github.ErrIssuanceRejected},
		{name: "status 200", status: 200, want: authz.ErrInstallationUnavailable},
		{name: "status 401", status: 401, want: authz.ErrInstallationUnavailable},
		{name: "status 404", status: 404, want: authz.ErrInstallationUnavailable},
		{name: "status 403 without rate limit headers", status: 403, want: authz.ErrInstallationUnavailable},
		{name: "status 403 with exhausted rate limit", status: 403, header: map[string]string{"X-Ratelimit-Remaining": "0"}, want: github.ErrTransientFailure},
		{name: "status 429", status: 429, want: github.ErrTransientFailure},
		{name: "status 500", status: 500, want: github.ErrTransientFailure},
	}
	for _, tt := range tests {
		t.Run(tt.name+" returns "+tt.want.Error(), func(t *testing.T) {
			// 201 以外の応答の本文にトークンがあっても読まない
			handler := func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("X-GitHub-Request-Id", requestID)
				for name, value := range tt.header {
					w.Header().Set(name, value)
				}
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, `{"token":"`+tokenMarker+`","message":"`+responseMarker+`"}`)
			}

			got, err := issue(t, newAPIStub(t, handler).client(t))

			assertIssueFailed(t, got, err, tt.want)
			for _, value := range []string{"status " + strconv.Itoa(tt.status), labeledRequestID} {
				if !strings.Contains(err.Error(), value) {
					t.Errorf("error message %q does not contain %q", err.Error(), value)
				}
			}
		})
	}
}

//trace:checks AC-13 AC-30
func TestIssue_CommunicationFailureReturnsErrTransientFailure(t *testing.T) {
	closeConnection := func(w http.ResponseWriter, _ *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("Hijack returned error: %v", err)
			return
		}
		_ = conn.Close()
	}
	partialBody := `{"token":"` + tokenMarker + `","message":"` + responseMarker
	truncateBody := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "1000")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, partialBody)
	}
	// net/http のサーバーは、要求の本文を読み切るまで切断を検知せず、r.Context() を終わらせない
	waitForCancel := func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	}
	// 発行 API が 201 とトークンまでを返した後、本文の残りを返さないまま期限が来る
	waitForCancelInBody := func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, partialBody)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}

	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{name: "connection closed before the response", handler: closeConnection},
		{name: "connection closed while reading the body", handler: truncateBody},
		{name: "deadline exceeded after the request is sent", handler: waitForCancel},
		{name: "deadline exceeded while reading the body", handler: waitForCancelInBody},
	}
	for _, tt := range tests {
		t.Run(tt.name+" returns ErrTransientFailure", func(t *testing.T) {
			client := newAPIStub(t, tt.handler).client(t)
			ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
			defer cancel()

			got, err := client.Issue(ctx, github.NewAppJWT(appJWTValue), tokenGrant(t))

			assertIssueFailed(t, got, err, github.ErrTransientFailure)
		})
	}
}

func TestIssue_DoesNotFollowRedirect(t *testing.T) {
	const redirectPath = "/redirected"
	body := marshal(t, recordedIssuance())
	// 転送先は同じサーバーにあり、従えば発行応答を取得できる
	var redirected atomic.Int32
	stub := newAPIStub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == redirectPath {
			redirected.Add(1)
			respondIssuance(body)(w, r)
			return
		}
		w.Header().Set("X-GitHub-Request-Id", requestID)
		http.Redirect(w, r, redirectPath, http.StatusTemporaryRedirect)
	})

	got, err := issue(t, stub.client(t))

	assertIssueFailed(t, got, err, authz.ErrInstallationUnavailable)
	if n := redirected.Load(); n != 0 {
		t.Errorf("redirect target received %d requests, want 0", n)
	}
}

func TestIssue_ZeroTokenGrantPanicsWithoutRequest(t *testing.T) {
	stub := newAPIStub(t, respondIssuance(marshal(t, recordedIssuance())))
	client := stub.client(t)
	defer func() {
		if recover() == nil {
			t.Error("Issue did not panic for the zero TokenGrant")
		}
		if stub.request.Load() != nil {
			t.Error("Issue sent a request for the zero TokenGrant")
		}
	}()

	_, _ = client.Issue(t.Context(), github.NewAppJWT(appJWTValue), authz.TokenGrant{})
}
