package githubapi_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/credential"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/githubapi"
)

const accessToken = "ghs_example"

// newClient は、TLS の httptest サーバーに向けた Client を作る。
func newClient(t *testing.T, handler http.Handler) *githubapi.Client {
	t.Helper()

	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	apiHost, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse(%q) error = %v", server.URL, err)
	}
	return githubapi.NewClient(githubapi.WithAPIHost(apiHost), githubapi.WithTransport(server.Client().Transport))
}

// recorder は、受け取った要求を記録して 204 を返す RoundTripper である。
type recorder struct{ requests []*http.Request }

func (r *recorder) RoundTrip(request *http.Request) (*http.Response, error) {
	r.requests = append(r.requests, request)
	return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody, Request: request}, nil
}

func TestClient_Revoke_SetsRevocationTimeout(t *testing.T) {
	t.Parallel()

	transport := &recorder{}
	client := githubapi.NewClient(githubapi.WithTransport(transport))

	start := time.Now()
	if _, err := client.Revoke(t.Context(), credential.NewInstallationAccessToken(accessToken)); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}

	deadline, ok := transport.requests[0].Context().Deadline()
	if !ok {
		t.Fatal("the request has no deadline")
	}
	if got := deadline.Sub(start); got < 4*time.Second || got > 6*time.Second {
		t.Errorf("deadline is %v after the start, want about 5s", got)
	}
}

func TestClient_Revoke(t *testing.T) {
	t.Parallel()

	var got *http.Request
	client := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		w.WriteHeader(http.StatusNoContent)
	}))

	status, err := client.Revoke(t.Context(), credential.NewInstallationAccessToken(accessToken))
	if err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}
	if status != http.StatusNoContent {
		t.Errorf("Revoke() status = %d, want %d", status, http.StatusNoContent)
	}
	if got.Method != http.MethodDelete {
		t.Errorf("method = %q, want %q", got.Method, http.MethodDelete)
	}
	if got.URL.Path != "/installation/token" {
		t.Errorf("path = %q, want %q", got.URL.Path, "/installation/token")
	}
	wantHeaders := map[string]string{
		"Authorization":        "Bearer " + accessToken,
		"Accept":               "application/vnd.github+json",
		"X-GitHub-Api-Version": "2026-03-10",
		"User-Agent":           "gates-issue-token",
	}
	for name, want := range wantHeaders {
		if v := got.Header.Get(name); v != want {
			t.Errorf("header %s = %q, want %q", name, v, want)
		}
	}
}

func TestClient_Revoke_ReturnsStatusWithoutErrorForNon204(t *testing.T) {
	t.Parallel()

	client := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
	}))

	status, err := client.Revoke(t.Context(), credential.NewInstallationAccessToken(accessToken))
	if err != nil {
		t.Fatalf("Revoke() error = %v, want nil", err)
	}
	if status != http.StatusUnauthorized {
		t.Errorf("Revoke() status = %d, want %d", status, http.StatusUnauthorized)
	}
}

func TestClient_Revoke_DoesNotFollowRedirect(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	client := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Redirect(w, r, "/elsewhere", http.StatusMovedPermanently)
	}))

	status, err := client.Revoke(t.Context(), credential.NewInstallationAccessToken(accessToken))
	if err != nil {
		t.Fatalf("Revoke() error = %v, want nil", err)
	}
	if status != http.StatusMovedPermanently {
		t.Errorf("Revoke() status = %d, want %d", status, http.StatusMovedPermanently)
	}
	if n := requests.Load(); n != 1 {
		t.Errorf("requests = %d, want 1", n)
	}
}

func TestClient_Revoke_TimeoutReturnsCommunicationFailure(t *testing.T) {
	t.Parallel()

	client := newClient(t, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	status, err := client.Revoke(ctx, credential.NewInstallationAccessToken(accessToken))
	if err == nil {
		t.Fatal("Revoke() error = nil, want an error")
	}
	if status != 0 {
		t.Errorf("Revoke() status = %d, want 0", status)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("errors.Is(err, context.DeadlineExceeded) = false, err = %v", err)
	}
	if want := "the request timed out"; err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestClient_Revoke_ConnectionFailureReturnsCommunicationFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.NotFoundHandler())
	apiHost, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse(%q) error = %v", server.URL, err)
	}
	server.Close()
	client := githubapi.NewClient(githubapi.WithAPIHost(apiHost))

	status, err := client.Revoke(t.Context(), credential.NewInstallationAccessToken(accessToken))
	if err == nil {
		t.Fatal("Revoke() error = nil, want an error")
	}
	if status != 0 {
		t.Errorf("Revoke() status = %d, want 0", status)
	}
	if want := "the request failed without a response"; err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestNewClient_DefaultsToGitHubAPI(t *testing.T) {
	t.Setenv("GITHUB_API_URL", "https://ignored.example")

	rec := &recorder{}
	client := githubapi.NewClient(githubapi.WithTransport(rec))

	if _, err := client.Revoke(t.Context(), credential.NewInstallationAccessToken(accessToken)); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}
	if len(rec.requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(rec.requests))
	}
	if got, want := rec.requests[0].URL.String(), "https://api.github.com/installation/token"; got != want {
		t.Errorf("URL = %q, want %q", got, want)
	}
}
