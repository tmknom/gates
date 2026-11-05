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
)

// lambdaRequestID は、Lambda の request ID の形をした相関 ID である。
const lambdaRequestID = "c6af9ac6-7b61-11e6-9a41-93e8deadbeef"

func decodeResponse(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var response map[string]any
	if err := json.Unmarshal(b, &response); err != nil {
		t.Fatalf("response %s is not a JSON object: %v", b, err)
	}
	return response
}

func assertKeys(t *testing.T, name string, object map[string]any, want ...string) {
	t.Helper()
	if got := slices.Sorted(maps.Keys(object)); !slices.Equal(got, slices.Sorted(slices.Values(want))) {
		t.Errorf("%s keys = %q, want %q", name, got, want)
	}
}

func TestEncodeFailure_WritesErrorCodeAndRequestID(t *testing.T) {
	response := decodeResponse(t, encodeFailure(lambdaRequestID, authorizationDenied))

	assertKeys(t, "failure response", response, "ok", "error", "request_id")
	if response["ok"] != false {
		t.Errorf("ok = %v, want false", response["ok"])
	}
	if response["request_id"] != lambdaRequestID {
		t.Errorf("request_id = %v, want %q", response["request_id"], lambdaRequestID)
	}
	want := map[string]any{"code": "authorization_denied", "message": "The requested token is not permitted."}
	if !reflect.DeepEqual(response["error"], want) {
		t.Errorf("error = %v, want %v", response["error"], want)
	}
}

func TestEncodeFailure_RequestIDThatIsNotUTF8Panics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("encodeFailure did not panic")
		}
	}()

	encodeFailure("\xff", internalError)
}

func TestEncodeSuccess_WritesVerifiedInstallationToken(t *testing.T) {
	token, err := issueInstallationToken(t, recordedIssuance(), http.StatusNoContent)
	if err != nil {
		t.Fatalf("IssueInstallationToken returned error: %v", err)
	}

	response := decodeResponse(t, encodeSuccess(lambdaRequestID, token))

	assertKeys(t, "success response", response, "ok", "token", "expires_at", "repositories", "permissions", "request_id")
	if response["ok"] != true {
		t.Errorf("ok = %v, want true", response["ok"])
	}
	if response["token"] != tokenMarker {
		t.Errorf("token = %v, want the token of the issuance response", response["token"])
	}
	expiresAt, ok := response["expires_at"].(string)
	if !ok {
		t.Fatalf("expires_at = %v, want a string", response["expires_at"])
	}
	// 有効期限を変えていないことは、表記ではなく時刻の一致で確かめる
	if got, err := time.Parse(time.RFC3339, expiresAt); err != nil || !got.Equal(time.Date(2026, 11, 3, 12, 18, 48, 0, time.UTC)) {
		t.Errorf("expires_at = %q, want the RFC 3339 form of 2026-11-03T12:18:48Z", expiresAt)
	}
	// 実効対象は発行応答の full_name の表記で返す。仕様は順序を定めないため、並べ替えて比べる
	repositories, _ := response["repositories"].([]any)
	got := make([]string, len(repositories))
	for i, name := range repositories {
		got[i] = fmt.Sprint(name)
	}
	slices.Sort(got)
	if want := []string{allowedOwnerName + "/example-repo", allowedOwnerName + "/repo-b"}; !slices.Equal(got, want) {
		t.Errorf("repositories = %v, want %v in any order", response["repositories"], want)
	}
	if want := map[string]any{"contents": "read", "metadata": "read"}; !reflect.DeepEqual(response["permissions"], want) {
		t.Errorf("permissions = %v, want %v", response["permissions"], want)
	}
	if response["request_id"] != lambdaRequestID {
		t.Errorf("request_id = %v, want %q", response["request_id"], lambdaRequestID)
	}
}
