package handler

import (
	"encoding/json/v2"
	"fmt"
	"maps"
	"net/http"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/tmknom/gates/internal/errorcode"
)

// lambdaRequestID は、Lambda の request ID の形をした相関 ID である。
const lambdaRequestID = "c6af9ac6-7b61-11e6-9a41-93e8deadbeef"

func decodeResponse(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("response %s is not a JSON object: %v", b, err)
	}
	return decoded
}

func assertKeys(t *testing.T, name string, object map[string]any, want ...string) {
	t.Helper()
	if got := slices.Sorted(maps.Keys(object)); !slices.Equal(got, slices.Sorted(slices.Values(want))) {
		t.Errorf("%s keys = %q, want %q", name, got, want)
	}
}

func TestEncodeFailure_WritesErrorCodeAndRequestID(t *testing.T) {
	got := decodeResponse(t, encodeFailure(lambdaRequestID, errorcode.AuthorizationDenied))

	assertKeys(t, "failure response", got, "ok", "error", "request_id")
	if got["ok"] != false {
		t.Errorf("ok = %v, want false", got["ok"])
	}
	if got["request_id"] != lambdaRequestID {
		t.Errorf("request_id = %v, want %q", got["request_id"], lambdaRequestID)
	}
	want := map[string]any{"code": "authorization_denied", "message": "The requested token is not permitted."}
	if !reflect.DeepEqual(got["error"], want) {
		t.Errorf("error = %v, want %v", got["error"], want)
	}
}

func TestEncodeFailure_RequestIDThatIsNotUTF8ReturnsZeroResponse(t *testing.T) {
	if got := encodeFailure("\xff", errorcode.InternalError); !got.IsZero() {
		t.Errorf("encodeFailure = %s, want the zero response", got)
	}
}

func TestEncodeSuccess_WritesVerifiedInstallationToken(t *testing.T) {
	token, err := issueInstallationToken(t, recordedIssuance(), http.StatusNoContent)
	if err != nil {
		t.Fatalf("IssueInstallationToken returned error: %v", err)
	}

	decoded := decodeResponse(t, encodeSuccess(lambdaRequestID, token))

	assertKeys(t, "success response", decoded, "ok", "token", "expires_at", "repositories", "permissions", "request_id")
	if decoded["ok"] != true {
		t.Errorf("ok = %v, want true", decoded["ok"])
	}
	if decoded["token"] != tokenMarker {
		t.Errorf("token = %v, want the token of the issuance response", decoded["token"])
	}
	expiresAt, ok := decoded["expires_at"].(string)
	if !ok {
		t.Fatalf("expires_at = %v, want a string", decoded["expires_at"])
	}
	// 有効期限を変えていないことは、表記ではなく時刻の一致で確かめる
	if got, err := time.Parse(time.RFC3339, expiresAt); err != nil || !got.Equal(time.Date(2026, 11, 3, 12, 18, 48, 0, time.UTC)) {
		t.Errorf("expires_at = %q, want the RFC 3339 form of 2026-11-03T12:18:48Z", expiresAt)
	}
	// 実効対象は発行応答の full_name の表記で返す。仕様は順序を定めないため、並べ替えて比べる
	repositories, _ := decoded["repositories"].([]any)
	got := make([]string, len(repositories))
	for i, name := range repositories {
		got[i] = fmt.Sprint(name)
	}
	slices.Sort(got)
	if want := []string{allowedOwnerName + "/example-repo", allowedOwnerName + "/repo-b"}; !slices.Equal(got, want) {
		t.Errorf("repositories = %v, want %v in any order", decoded["repositories"], want)
	}
	if want := map[string]any{"contents": "read", "metadata": "read"}; !reflect.DeepEqual(decoded["permissions"], want) {
		t.Errorf("permissions = %v, want %v", decoded["permissions"], want)
	}
	if decoded["request_id"] != lambdaRequestID {
		t.Errorf("request_id = %v, want %q", decoded["request_id"], lambdaRequestID)
	}
}
