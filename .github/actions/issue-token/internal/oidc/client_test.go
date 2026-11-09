package oidc_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/credential"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/oidc"
)

const requestToken = "request-token"

// newClient は、TLS の httptest サーバーに向けた Client を作る。pathAndQuery はサーバーの URL に足すパスとクエリである。
func newClient(t *testing.T, handler http.Handler, pathAndQuery string) *oidc.Client {
	t.Helper()

	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	client, err := oidc.NewClient(server.URL+pathAndQuery, credential.NewOIDCRequestToken(requestToken), oidc.WithTransport(server.Client().Transport))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client
}

func TestNewClient_RejectsURL(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"http URL":               "http://token.example/token?api-version=2.0",
		"unparsable URL":         "https://exa mple.com/token",
		"https URL without host": "https:///token",
	}
	for name, requestURL := range tests {
		t.Run(name+" returns an error", func(t *testing.T) {
			t.Parallel()

			client, err := oidc.NewClient(requestURL, credential.NewOIDCRequestToken(requestToken))
			if err == nil {
				t.Fatalf("NewClient(%q) error = nil, client = %v", requestURL, client)
			}
		})
	}
}
