package revoke_test

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/githubapi"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/revoke"
)

const token = "ghs_example"

// authorizationRecorder は、サーバーが受けた要求の Authorization を、handler の goroutine とテストの間で守って記録する。
type authorizationRecorder struct {
	mu     sync.Mutex
	values []string
}

func (a *authorizationRecorder) add(value string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.values = append(a.values, value)
}

func (a *authorizationRecorder) get() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Clone(a.values)
}

// newGitHubServer は、受けた要求の Authorization を記録して status を返す TLS の httptest サーバーと、そこへ向ける Option を作る。
func newGitHubServer(t *testing.T, status int) (*authorizationRecorder, []githubapi.Option) {
	t.Helper()

	received := &authorizationRecorder{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.add(r.Header.Get("Authorization"))
		w.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	apiHost, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse(%q) error = %v", server.URL, err)
	}
	return received, []githubapi.Option{githubapi.WithAPIHost(apiHost), githubapi.WithTransport(server.Client().Transport)}
}

func TestRun_WithoutTokenDoesNothing(t *testing.T) {
	t.Parallel()

	authorizations, opts := newGitHubServer(t, http.StatusNoContent)
	var stdout bytes.Buffer

	err := revoke.Run(t.Context(), revoke.Config{Token: "", Stdout: &stdout, GitHub: opts})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if len(authorizations.get()) != 0 {
		t.Errorf("requests = %d, want 0", len(authorizations.get()))
	}
}

//trace:checks AC-31
func TestRun_MasksTheTokenBeforeRevokingWithIt(t *testing.T) {
	t.Parallel()

	authorizations, opts := newGitHubServer(t, http.StatusNoContent)
	var stdout bytes.Buffer

	err := revoke.Run(t.Context(), revoke.Config{Token: token, Stdout: &stdout, GitHub: opts})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 2 || lines[0] != "::add-mask::"+token {
		t.Fatalf("stdout lines = %q, want the mask of the token first and the result of the revocation", lines)
	}
	if want := []string{"Bearer " + token}; len(authorizations.get()) != 1 || authorizations.get()[0] != want[0] {
		t.Errorf("Authorization of the requests = %q, want %q", authorizations.get(), want)
	}
}

var errWrite = errors.New("write failed")

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }

func TestRun_RevokesAndReturnsTheWriteFailure(t *testing.T) {
	t.Parallel()

	authorizations, opts := newGitHubServer(t, http.StatusNoContent)

	err := revoke.Run(t.Context(), revoke.Config{Token: token, Stdout: failingWriter{}, GitHub: opts})
	if !errors.Is(err, errWrite) {
		t.Errorf("Run() error = %v, want %v", err, errWrite)
	}
	if len(authorizations.get()) != 1 {
		t.Errorf("requests = %d, want 1 because the revocation does not depend on the log", len(authorizations.get()))
	}
}
