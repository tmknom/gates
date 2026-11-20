package request_test

import (
	"testing"

	"github.com/tmknom/gates/internal/request"
)

func TestTokenRequest_RepositoriesCannotBeModifiedByCaller(t *testing.T) {
	_, req, err := request.Validate([]byte(payloadWith("repositories", `["org/repo-a","org/repo-b"]`)))
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	got := req.Repositories()
	got[0] = got[1]
	assertRepositories(t, req.Repositories(), []string{"org/repo-a", "org/repo-b"})
}
