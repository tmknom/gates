package github_test

import (
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/github"
)

const (
	recordedInstallationID = 12345678
	recordedAccountID      = 583231
	allowedOwnerName       = "allowed-owner"
	requestID              = "ABCD:1234:5678:9ABC:DEF0"

	// responseMarker は、応答本文がエラー文言に漏れていないことを確かめる目印である。
	responseMarker = "response-body-marker"

	// installationSizeLimit と installationTimeout は Lambda Design の GitHub Client と Call Time Budget が定める値である。
	installationSizeLimit = 64 * 1024
	installationTimeout   = 5 * time.Second
)

// recordedInstallation は、調査記録の Installation の応答を、ケースごとに書き換えられる形で返す。
func recordedInstallation() map[string]any {
	return map[string]any{
		"id":        recordedInstallationID,
		"client_id": "Iv23liEXAMPLE0000000",
		"account": map[string]any{
			"login": "octocat",
			"id":    recordedAccountID,
			"type":  "User",
		},
		"repository_selection": "selected",
		"app_id":               123456,
		"target_id":            recordedAccountID,
		"target_type":          "User",
		"permissions":          map[string]any{"contents": "read", "metadata": "read"},
		"events":               []any{},
		"created_at":           "2026-10-28T04:32:35.000Z",
		"suspended_by":         nil,
		"suspended_at":         nil,
	}
}

func marshal(t *testing.T, value any) string {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	return string(b)
}

func respondBody(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-GitHub-Request-Id", requestID)
		_, _ = io.WriteString(w, body)
	}
}

func respondStatus(status int, header map[string]string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-GitHub-Request-Id", requestID)
		for name, value := range header {
			w.Header().Set(name, value)
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, responseMarker)
	}
}

// apiStub は GitHub API の代わりに応答する TLS サーバーである。
type apiStub struct {
	server *httptest.Server
	// サーバーの goroutine が書き、テストが読む
	request atomic.Pointer[http.Request]
}

func newAPIStub(t *testing.T, handler http.HandlerFunc) *apiStub {
	t.Helper()
	stub := &apiStub{}
	stub.server = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stub.request.Store(r)
		handler(w, r)
	}))
	// 切断を試すハンドラで、サーバーのログがテストの出力に混ざらないようにする
	stub.server.Config.ErrorLog = log.New(io.Discard, "", 0)
	stub.server.StartTLS()
	t.Cleanup(stub.server.Close)
	return stub
}

func (s *apiStub) client(t *testing.T) *github.Client {
	t.Helper()
	apiHost, err := url.Parse(s.server.URL)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	return github.NewClient(github.WithAPIHost(apiHost), github.WithTransport(s.server.Client().Transport))
}

func allowedOwner(t *testing.T, ownerType authz.OwnerType) authz.AllowedOwner {
	t.Helper()
	owner, err := authz.NewAllowedOwner(recordedAccountID, allowedOwnerName, ownerType)
	if err != nil {
		t.Fatalf("NewAllowedOwner returned error: %v", err)
	}
	return owner
}

func newInstallation(t *testing.T, suspendedAt time.Time) authz.Installation {
	t.Helper()
	installation, err := authz.NewInstallation(recordedInstallationID, recordedAccountID, suspendedAt)
	if err != nil {
		t.Fatalf("NewInstallation returned error: %v", err)
	}
	return installation
}

func resolve(t *testing.T, client *github.Client) (authz.Installation, error) {
	t.Helper()
	return client.ResolveInstallation(t.Context(), github.NewAppJWT(appJWTValue), allowedOwner(t, authz.OwnerTypeUser))
}

// assertRejected は失敗の種別と、どの失敗でも GitHub App JWT、応答本文、許可オーナー名がエラー文言に入らないことを確かめる。
func assertRejected(t *testing.T, got authz.Installation, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("ResolveInstallation error = %v, want %v", err, want)
	}
	other := map[error]error{authz.ErrInstallationUnavailable: github.ErrTransientFailure, github.ErrTransientFailure: authz.ErrInstallationUnavailable}[want]
	if errors.Is(err, other) {
		t.Errorf("ResolveInstallation error = %v, also matches %v", err, other)
	}
	if got != (authz.Installation{}) {
		t.Errorf("ResolveInstallation = %+v, want zero Installation", got)
	}
	for _, value := range []string{appJWTValue, responseMarker, allowedOwnerName} {
		if strings.Contains(err.Error(), value) {
			t.Errorf("error message %q contains %q", err.Error(), value)
		}
	}
}

func TestResolveInstallation_RequestsOwnerTypeEndpoint(t *testing.T) {
	tests := []struct {
		name      string
		ownerType authz.OwnerType
		path      string
	}{
		{name: "Organization gets the organization installation", ownerType: authz.OwnerTypeOrganization, path: "/orgs/" + allowedOwnerName + "/installation"},
		{name: "User gets the user installation", ownerType: authz.OwnerTypeUser, path: "/users/" + allowedOwnerName + "/installation"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := newAPIStub(t, respondBody(marshal(t, recordedInstallation())))

			if _, err := stub.client(t).ResolveInstallation(t.Context(), github.NewAppJWT(appJWTValue), allowedOwner(t, tt.ownerType)); err != nil {
				t.Fatalf("ResolveInstallation returned error: %v", err)
			}
			if request := stub.request.Load(); request.Method != http.MethodGet || request.URL.Path != tt.path {
				t.Errorf("request = %s %s, want GET %s", request.Method, request.URL.Path, tt.path)
			}
		})
	}
}

func TestResolveInstallation_SendsRequiredHeaders(t *testing.T) {
	request := sentRequest(t)
	wantHeader := map[string]string{
		"Authorization":        "Bearer " + appJWTValue,
		"Accept":               "application/vnd.github+json",
		"X-GitHub-Api-Version": "2026-03-10",
		"User-Agent":           "gates",
	}
	for name, want := range wantHeader {
		if got := request.Header.Values(name); len(got) != 1 || got[0] != want {
			t.Errorf("request header %s = %q, want [%q]", name, got, want)
		}
	}
}

// roundTripperFunc は、要求を外へ送らずに応答を返す Transport である。
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

// sentRequest は、既定の API Host のまま ResolveInstallation を呼び、外へ送らずに受け取った要求を返す。
func sentRequest(t *testing.T) *http.Request {
	t.Helper()
	var sent *http.Request
	transport := roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		sent = request
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader(marshal(t, recordedInstallation()))),
		}, nil
	})
	if _, err := resolve(t, github.NewClient(github.WithTransport(transport))); err != nil {
		t.Fatalf("ResolveInstallation returned error: %v", err)
	}
	return sent
}

func TestResolveInstallation_SendsToGitHubAPIHost(t *testing.T) {
	if got, want := sentRequest(t).URL.String(), "https://api.github.com/users/"+allowedOwnerName+"/installation"; got != want {
		t.Errorf("request URL = %q, want %q", got, want)
	}
}

func TestResolveInstallation_WaitsAtMostInstallationTimeout(t *testing.T) {
	deadline, ok := sentRequest(t).Context().Deadline()
	if !ok {
		t.Fatal("request has no deadline")
	}
	if remaining := time.Until(deadline); remaining > installationTimeout || remaining < installationTimeout/2 {
		t.Errorf("remaining time until the deadline = %v, want about %v", remaining, installationTimeout)
	}
}

func TestResolveInstallation_ReadsSuspendedAt(t *testing.T) {
	suspended := recordedInstallation()
	suspended["suspended_at"] = "2026-11-03T11:17:03Z"
	suspended["suspended_by"] = map[string]any{"login": "octocat", "id": recordedAccountID}

	tests := []struct {
		name string
		body map[string]any
		want time.Time
	}{
		{name: "null suspended_at is not suspended", body: recordedInstallation(), want: time.Time{}},
		{name: "time suspended_at is suspended", body: suspended, want: time.Date(2026, 11, 3, 11, 17, 3, 0, time.UTC)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolve(t, newAPIStub(t, respondBody(marshal(t, tt.body))).client(t))
			if err != nil {
				t.Fatalf("ResolveInstallation returned error: %v", err)
			}
			if want := newInstallation(t, tt.want); got != want {
				t.Errorf("ResolveInstallation = %+v, want %+v", got, want)
			}
		})
	}
}

func TestResolveInstallation_UnreadableInstallationReturnsErrInstallationUnavailable(t *testing.T) {
	with := func(change func(body map[string]any)) func(t *testing.T) string {
		return func(t *testing.T) string {
			body := recordedInstallation()
			change(body)
			return marshal(t, body)
		}
	}
	raw := func(body string) func(*testing.T) string { return func(*testing.T) string { return body } }

	tests := []struct {
		name string
		body func(t *testing.T) string
	}{
		{name: "missing id", body: with(func(body map[string]any) { delete(body, "id") })},
		{name: "zero id", body: with(func(body map[string]any) { body["id"] = 0 })},
		{name: "missing account id", body: with(func(body map[string]any) { delete(body["account"].(map[string]any), "id") })},
		// OpenAPI 定義で account は null を取り得る
		{name: "null account", body: with(func(body map[string]any) { body["account"] = nil })},
		{name: "missing suspended_at", body: with(func(body map[string]any) { delete(body, "suspended_at") })},
		{name: "non time suspended_at", body: with(func(body map[string]any) { body["suspended_at"] = responseMarker })},
		{name: "zero time suspended_at", body: with(func(body map[string]any) { body["suspended_at"] = "0001-01-01T00:00:00Z" })},
		{name: "non JSON body", body: raw("<html>" + responseMarker)},
		{name: "duplicate id", body: raw(`{"id":1,"id":12345678,"account":{"id":583231},"suspended_at":null,"x":"` + responseMarker + `"}`)},
	}
	for _, tt := range tests {
		t.Run(tt.name+" returns ErrInstallationUnavailable", func(t *testing.T) {
			got, err := resolve(t, newAPIStub(t, respondBody(tt.body(t))).client(t))

			assertRejected(t, got, err, authz.ErrInstallationUnavailable)
			if !strings.Contains(err.Error(), requestID) {
				t.Errorf("error message %q does not contain the request id", err.Error())
			}
		})
	}
}

func TestResolveInstallation_StatusFailure(t *testing.T) {
	tests := []struct {
		name   string
		status int
		header map[string]string
		want   error
	}{
		{name: "status 401", status: 401, want: authz.ErrInstallationUnavailable},
		{name: "status 404", status: 404, want: authz.ErrInstallationUnavailable},
		{name: "status 410", status: 410, want: authz.ErrInstallationUnavailable},
		{name: "status 499", status: 499, want: authz.ErrInstallationUnavailable},
		{name: "status 403 without rate limit headers", status: 403, want: authz.ErrInstallationUnavailable},
		{name: "status 403 with remaining rate limit", status: 403, header: map[string]string{"X-Ratelimit-Remaining": "5"}, want: authz.ErrInstallationUnavailable},
		{name: "status 403 with exhausted rate limit", status: 403, header: map[string]string{"X-Ratelimit-Remaining": "0"}, want: github.ErrTransientFailure},
		{name: "status 403 with retry-after", status: 403, header: map[string]string{"Retry-After": "60"}, want: github.ErrTransientFailure},
		{name: "status 429", status: 429, want: github.ErrTransientFailure},
		{name: "status 500", status: 500, want: github.ErrTransientFailure},
	}
	for _, tt := range tests {
		t.Run(tt.name+" returns "+tt.want.Error(), func(t *testing.T) {
			got, err := resolve(t, newAPIStub(t, respondStatus(tt.status, tt.header)).client(t))

			assertRejected(t, got, err, tt.want)
			for _, value := range []string{"status " + strconv.Itoa(tt.status), requestID} {
				if !strings.Contains(err.Error(), value) {
					t.Errorf("error message %q does not contain %q", err.Error(), value)
				}
			}
		})
	}
}

func TestResolveInstallation_CommunicationFailureReturnsErrTransientFailure(t *testing.T) {
	closeConnection := func(w http.ResponseWriter, _ *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			panic(err)
		}
		_ = conn.Close()
	}
	truncateBody := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "1000")
		_, _ = io.WriteString(w, `{"id":`+responseMarker)
	}
	waitForCancel := func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() }

	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{name: "connection closed before the response", handler: closeConnection},
		{name: "connection closed while reading the body", handler: truncateBody},
		{name: "deadline exceeded", handler: waitForCancel},
	}
	for _, tt := range tests {
		t.Run(tt.name+" returns ErrTransientFailure", func(t *testing.T) {
			client := newAPIStub(t, tt.handler).client(t)
			ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
			defer cancel()

			got, err := client.ResolveInstallation(ctx, github.NewAppJWT(appJWTValue), allowedOwner(t, authz.OwnerTypeUser))

			assertRejected(t, got, err, github.ErrTransientFailure)
		})
	}
}

// padded は JSON の後ろに空白を足し、復号できる本文のまま大きさだけを size に揃える。
func padded(body string, size int) string {
	return body + strings.Repeat(" ", size-len(body))
}

func TestResolveInstallation_ResponseSizeLimit(t *testing.T) {
	t.Run("body at the limit is accepted", func(t *testing.T) {
		body := padded(marshal(t, recordedInstallation()), installationSizeLimit)

		if _, err := resolve(t, newAPIStub(t, respondBody(body)).client(t)); err != nil {
			t.Fatalf("ResolveInstallation returned error: %v", err)
		}
	})
	t.Run("body over the limit returns ErrInstallationUnavailable", func(t *testing.T) {
		body := padded(marshal(t, recordedInstallation()), installationSizeLimit+1)

		got, err := resolve(t, newAPIStub(t, respondBody(body)).client(t))

		assertRejected(t, got, err, authz.ErrInstallationUnavailable)
	})
}

func TestResolveInstallation_DoesNotFollowRedirect(t *testing.T) {
	const redirectPath = "/redirected"
	body := marshal(t, recordedInstallation())
	// 転送先は同じサーバーにあり、従えば Installation を取得できる
	var redirected atomic.Int32
	stub := newAPIStub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == redirectPath {
			redirected.Add(1)
			_, _ = io.WriteString(w, body)
			return
		}
		w.Header().Set("X-GitHub-Request-Id", requestID)
		http.Redirect(w, r, redirectPath, http.StatusFound)
	})

	got, err := resolve(t, stub.client(t))

	assertRejected(t, got, err, authz.ErrInstallationUnavailable)
	if n := redirected.Load(); n != 0 {
		t.Errorf("redirect target received %d requests, want 0", n)
	}
}

func TestResolveInstallation_ZeroAllowedOwnerPanics(t *testing.T) {
	client := newAPIStub(t, respondBody(marshal(t, recordedInstallation()))).client(t)
	defer func() {
		if recover() == nil {
			t.Error("ResolveInstallation did not panic for the zero AllowedOwner")
		}
	}()

	_, _ = client.ResolveInstallation(t.Context(), github.NewAppJWT(appJWTValue), authz.AllowedOwner{})
}
