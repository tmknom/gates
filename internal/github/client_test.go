package github_test

import (
	"net/http"
	"testing"
)

func TestClient_OpensConnectionForEachRequest(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("GET /users/"+allowedOwnerName+"/installation", respondBody(marshal(t, recordedInstallation())))
	mux.Handle("POST /app/installations/{id}/access_tokens", respondIssuance(marshal(t, recordedIssuance())))
	stub := newAPIStub(t, mux.ServeHTTP)
	client := stub.client(t)

	if _, err := resolve(t, client); err != nil {
		t.Fatalf("ResolveInstallation returned error: %v", err)
	}
	if _, err := issue(t, client); err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	if got := stub.newConnections.Load(); got != 2 {
		t.Errorf("new connections = %d, want 2", got)
	}
}
