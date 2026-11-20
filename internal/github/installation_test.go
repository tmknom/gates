package github_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/github"
)

const (
	// installationSizeLimit と installationTimeout は Lambda Design の GitHub Client と Call Time Budget が定める値である。
	installationSizeLimit = 64 * 1024
	installationTimeout   = 5 * time.Second
)

//trace:checks AC-AUTHZ-007
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
	zeroTimeSuspended := recordedInstallation()
	zeroTimeSuspended["suspended_at"] = "0001-01-01T00:00:00Z"

	tests := []struct {
		name string
		body map[string]any
		want bool
	}{
		{name: "null suspended_at is not suspended", body: recordedInstallation(), want: false},
		{name: "time suspended_at is suspended", body: suspended, want: true},
		{name: "zero time suspended_at is suspended", body: zeroTimeSuspended, want: true},
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
		{name: "non JSON body", body: raw("<html>" + responseMarker)},
		{name: "duplicate id", body: raw(`{"id":1,"id":12345678,"account":{"id":583231},"suspended_at":null,"x":"` + responseMarker + `"}`)},
	}
	for _, tt := range tests {
		t.Run(tt.name+" returns ErrInstallationUnavailable", func(t *testing.T) {
			got, err := resolve(t, newAPIStub(t, respondBody(tt.body(t))).client(t))

			assertRejected(t, got, err, authz.ErrInstallationUnavailable)
			if !strings.Contains(err.Error(), labeledRequestID) {
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
			for _, value := range []string{"status " + strconv.Itoa(tt.status), labeledRequestID} {
				if !strings.Contains(err.Error(), value) {
					t.Errorf("error message %q does not contain %q", err.Error(), value)
				}
			}
		})
	}
}

//trace:checks AC-QUALITY-001
func TestResolveInstallation_CommunicationFailureReturnsErrTransientFailure(t *testing.T) {
	closeConnection := func(w http.ResponseWriter, _ *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("Hijack returned error: %v", err)
			return
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

func TestResolveInstallation_ZeroAppJWTPanicsWithoutRequest(t *testing.T) {
	stub := newAPIStub(t, respondBody(marshal(t, recordedInstallation())))
	client := stub.client(t)
	defer func() {
		got := recover()
		if got == nil {
			t.Error("ResolveInstallation did not panic for the zero AppJWT")
		}
		// nil の参照外しではなく、ゼロ値の検査で止めることを確かめる
		if _, ok := got.(runtime.Error); ok {
			t.Errorf("ResolveInstallation panicked with a runtime error: %v", got)
		}
		if stub.request.Load() != nil {
			t.Error("ResolveInstallation sent a request for the zero AppJWT")
		}
	}()

	_, _ = client.ResolveInstallation(t.Context(), github.AppJWT{}, allowedOwner(t, authz.OwnerTypeOrganization))
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
