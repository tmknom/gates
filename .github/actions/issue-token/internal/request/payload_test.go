package request_test

import (
	"testing"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/credential"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/request"
)

func TestPayload_EncodesJWTAsJSONString(t *testing.T) {
	t.Parallel()

	tokenRequest, err := request.Parse("example-org/repo-a", "contents:read")
	if err != nil {
		t.Fatal(err)
	}

	got := string(tokenRequest.Payload(credential.NewOIDCJWT(`header"quote\back`)))

	want := `{"version":1,"id_token":"header\"quote\\back","repositories":["example-org/repo-a"],"permissions":{"contents":"read"}}`
	if got != want {
		t.Errorf("Payload() = %s, want %s", got, want)
	}
}

func TestPayload_InvalidUTF8JWTPanics(t *testing.T) {
	t.Parallel()

	tokenRequest, err := request.Parse("example-org/repo-a", "contents:read")
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if recover() == nil {
			t.Error("Payload() did not panic")
		}
	}()
	tokenRequest.Payload(credential.NewOIDCJWT("jwt-\xff"))
}
