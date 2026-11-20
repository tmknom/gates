package github_test

import (
	"crypto/x509"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/tmknom/gates/internal/github"
)

const (
	requestID = "ABCD:1234:5678:9ABC:DEF0"
	// labeledRequestID は、エラー文言の GitHub の request ID を、相関 ID と見分けるラベル付きで表す
	labeledRequestID = `github request id "` + requestID + `"`

	// responseMarker は、応答本文がエラー文言に漏れていないことを確かめる目印である。
	responseMarker = "response-body-marker"
)

// apiStub は GitHub API の代わりに応答する TLS サーバーである。
type apiStub struct {
	server *httptest.Server
	// サーバーの goroutine が書き、テストが読む
	request        atomic.Pointer[http.Request]
	newConnections atomic.Int32
}

// roundTripperFunc は、要求を外へ送らずに応答を返す Transport である。
type roundTripperFunc func(*http.Request) (*http.Response, error)

func newAPIStub(t *testing.T, handler http.HandlerFunc) *apiStub {
	t.Helper()
	stub := &apiStub{}
	stub.server = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stub.request.Store(r)
		handler(w, r)
	}))
	stub.server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			stub.newConnections.Add(1)
		}
	}
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
	rootCAs := x509.NewCertPool()
	rootCAs.AddCert(s.server.Certificate())
	return github.NewClient(github.WithAPIHost(apiHost), github.WithRootCAs(rootCAs))
}

func respondBody(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-GitHub-Request-Id", requestID)
		_, _ = io.WriteString(w, body)
	}
}

func respondIssuance(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-GitHub-Request-Id", requestID)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, body)
	}
}

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }
