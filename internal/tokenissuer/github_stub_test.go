package tokenissuer

import (
	"context"
	"encoding/base64"
	"io"
	"log"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/tmknom/gates/internal/github"
)

// gitHubStub は GitHub API の代わりに応答する TLS サーバーで、受け取った要求の Authorization を経路ごとに記録する。
type gitHubStub struct {
	server *httptest.Server
	mu     sync.Mutex
	// authorizations は、経路ごとに受け取った Authorization を受け取った順に持つ
	authorizations map[string][]string
	// revocationContexts は、GitHub Client が失効の要求に付けた ctx を、送ろうとした順に持つ。サーバーに届かなかった要求も含む
	revocationContexts []context.Context
}

const (
	// revocationRequestID は、失効の応答の GitHub の request ID で、発行応答の request ID と見分ける。
	revocationRequestID = "REVO:1234:5678:9ABC:DEF0"

	installationPath = "GET /users/" + allowedOwnerName + "/installation"
	issuancePath     = "POST /app/installations/12345678/access_tokens"
	revocationPath   = "DELETE /installation/token"
	// repositoryPolicyPath は、リポジトリポリシーの取得の経路で、どの対象の取得も受け取る
	repositoryPolicyPath = "GET /repos/{owner}/{repo}/contents/.github/gates-policy.yml"
)

// documentMarker は、リポジトリポリシーの文書の内容がエラー文言に漏れていないことを確かめる目印である。
const documentMarker = "document-marker"

// newGitHubStub は、Installation の取得、発行、失効、リポジトリポリシーの取得の経路に、記録した応答を返すハンドラを置く。handlers は経路ごとにハンドラを差し替える。
func newGitHubStub(t *testing.T, handlers map[string]http.HandlerFunc) *gitHubStub {
	t.Helper()
	routes := map[string]http.HandlerFunc{
		installationPath: respond(http.StatusOK, requestID, marshal(t, map[string]any{"id": installationID, "account": map[string]any{"id": allowedOwnerID}, "suspended_at": nil})),
		issuancePath:     respond(http.StatusCreated, requestID, marshal(t, recordedIssuance())),
		revocationPath:   respond(http.StatusNoContent, revocationRequestID, ""),
		repositoryPolicyPath: func(w http.ResponseWriter, r *http.Request) {
			respond(http.StatusOK, requestID, repositoryPolicyFile(t, r.PathValue("repo")))(w, r)
		},
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

// roundTripperFunc は、GitHub Client の要求を、サーバーへ送る前に観測するための Transport である。
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

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

// repositoryPolicyFile は、記録した Contents API のファイルの応答の形で、repo の文書を返す本文である。
func repositoryPolicyFile(t *testing.T, repo string) string {
	t.Helper()
	document := documentOf(repo)
	return marshal(t, map[string]any{
		"type":     "file",
		"encoding": "base64",
		"size":     len(document),
		"name":     "gates-policy.yml",
		"path":     ".github/gates-policy.yml",
		"content":  base64.StdEncoding.EncodeToString([]byte(document)),
	})
}

// documentOf は、どの対象の文書かを見分けられるリポジトリポリシーの文書である。
func documentOf(repo string) string {
	return "callers: []\n# " + documentMarker + " " + repo + "\n"
}
