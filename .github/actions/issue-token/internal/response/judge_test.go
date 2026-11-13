package response_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/awsapi"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/response"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return body
}

// ok は、SDK のエラーも FunctionError もない 200 の Invoke の結果を作る。
func ok(payload []byte) awsapi.InvokeResult {
	return awsapi.InvokeResult{StatusCode: 200, Payload: payload}
}

// success は、判定を通る応答を作る。Items のテストの準備に使う。
func success(t *testing.T, name string) response.Success {
	t.Helper()
	got, err := response.Judge(ok(fixture(t, name)), nil)
	if err != nil {
		t.Fatalf("Judge(%s) error = %v, want nil", name, err)
	}
	return got
}

func TestJudge_ExtractsToken(t *testing.T) {
	t.Parallel()

	// 500 文字で + / = を含み、= で終わる
	long := strings.Repeat("ab+/", 124) + "ab+="
	longPayload := fmt.Appendf(nil, `{"ok":true,"token":%q,"request_id":"abc"}`, long)

	tests := map[string]struct {
		payload []byte
		want    string
	}{
		"plain token":                   {payload: fixture(t, "success.json"), want: "ghs_exampleToken123"},
		"500 characters with + / and =": {payload: longPayload, want: long},
		"unknown top-level field":       {payload: fixture(t, "success_unknown_field.json"), want: "ghs_exampleToken123"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := response.Judge(ok(tt.payload), nil)
			if err != nil {
				t.Fatalf("Judge() error = %v, want nil", err)
			}
			if token := got.Token().Expose(); token != tt.want {
				t.Errorf("Token() = %q, want %q", token, tt.want)
			}
		})
	}
}

//trace:checks AC-16 AC-17
func TestJudge_FailsWithAFixedMessage(t *testing.T) {
	t.Parallel()

	failedOK := ok(fixture(t, "success.json"))
	notJSON := func(payload []byte) string {
		return fmt.Sprintf("the Lambda response is not valid JSON (%d bytes)", len(payload))
	}
	noBool := func(payload []byte) string {
		return fmt.Sprintf("the Lambda response has no boolean ok (%d bytes)", len(payload))
	}
	invalidUTF8 := []byte("{\"ok\":true,\"token\":\"ghs_\xff\",\"request_id\":\"abc\"}")
	topString := []byte(`"ok"`)

	tests := map[string]struct {
		result    awsapi.InvokeResult
		invokeErr error
		want      string
	}{
		// Judge は invokeErr の種類で分岐せず、その文言に段階の語を前置するだけのため、種類ごとのケースを置かない
		"SDK error": {
			result:    failedOK,
			invokeErr: errors.New("cause"),
			want:      "the Lambda Invoke failed: cause",
		},
		"StatusCode 202": {
			result: awsapi.InvokeResult{StatusCode: 202, Payload: failedOK.Payload},
			want:   "the Lambda Invoke returned StatusCode 202",
		},
		"StatusCode 204": {
			result: awsapi.InvokeResult{StatusCode: 204},
			want:   "the Lambda Invoke returned StatusCode 204",
		},
		"StatusCode 0": {
			result: awsapi.InvokeResult{},
			want:   "the Lambda Invoke returned StatusCode 0",
		},
		"StatusCode is judged before FunctionError": {
			result: awsapi.InvokeResult{StatusCode: 202, FunctionError: "Unhandled", Payload: fixture(t, "function_error.json")},
			want:   "the Lambda Invoke returned StatusCode 202",
		},
		"FunctionError with errorType": {
			result: awsapi.InvokeResult{StatusCode: 200, FunctionError: "Unhandled", Payload: fixture(t, "function_error.json")},
			want:   `the Lambda function returned FunctionError "Unhandled" with errorType "Runtime.ExitError"`,
		},
		"FunctionError with a payload that is not JSON": {
			result: awsapi.InvokeResult{StatusCode: 200, FunctionError: "Unhandled", Payload: fixture(t, "not_json.txt")},
			want:   `the Lambda function returned FunctionError "Unhandled"`,
		},
		"FunctionError with errorType that is not a string": {
			result: awsapi.InvokeResult{StatusCode: 200, FunctionError: "Unhandled", Payload: fixture(t, "function_error_number_type.json")},
			want:   `the Lambda function returned FunctionError "Unhandled"`,
		},
		"FunctionError containing ##[": {
			result: awsapi.InvokeResult{StatusCode: 200, FunctionError: "a##[error]b", Payload: fixture(t, "not_json.txt")},
			want:   "the Lambda function returned FunctionError",
		},
		"FunctionError with errorType containing ##[": {
			result: awsapi.InvokeResult{StatusCode: 200, FunctionError: "Unhandled", Payload: []byte(`{"errorType":"a##[error]b"}`)},
			want:   `the Lambda function returned FunctionError "Unhandled"`,
		},
		"FunctionError is judged before the payload": {
			result: awsapi.InvokeResult{StatusCode: 200, FunctionError: "Unhandled", Payload: failedOK.Payload},
			want:   `the Lambda function returned FunctionError "Unhandled"`,
		},
		"payload that is not JSON": {
			result: ok(fixture(t, "not_json.txt")),
			want:   notJSON(fixture(t, "not_json.txt")),
		},
		"empty payload": {
			result: ok(nil),
			want:   notJSON(nil),
		},
		"duplicate name": {
			result: ok(fixture(t, "duplicate_name.json")),
			want:   notJSON(fixture(t, "duplicate_name.json")),
		},
		"trailing data": {
			result: ok(fixture(t, "trailing_data.json")),
			want:   notJSON(fixture(t, "trailing_data.json")),
		},
		"string that is not UTF-8": {
			result: ok(invalidUTF8),
			want:   notJSON(invalidUTF8),
		},
		"array at the top level": {
			result: ok(fixture(t, "top_array.json")),
			want:   notJSON(fixture(t, "top_array.json")),
		},
		"string at the top level": {
			result: ok(topString),
			want:   notJSON(topString),
		},
		"null at the top level": {
			result: ok(fixture(t, "top_null.json")),
			want:   noBool(fixture(t, "top_null.json")),
		},
		"ok is a string": {
			result: ok(fixture(t, "ok_string.json")),
			want:   noBool(fixture(t, "ok_string.json")),
		},
		"ok is null": {
			result: ok(fixture(t, "ok_null.json")),
			want:   noBool(fixture(t, "ok_null.json")),
		},
		"ok is missing": {
			result: ok(fixture(t, "ok_missing.json")),
			want:   noBool(fixture(t, "ok_missing.json")),
		},
		"failure response": {
			result: ok(fixture(t, "failure.json")),
			want:   `GATES returned a failure response: error code "authorization_denied", message "The requested token is not permitted.", request id "abc"`,
		},
		"failure response with a numeric code": {
			result: ok(fixture(t, "failure_number_code.json")),
			want:   `GATES returned a failure response: message "The requested token is not permitted.", request id "abc"`,
		},
		"failure response with a numeric request id": {
			result: ok(fixture(t, "failure_number_request_id.json")),
			want:   `GATES returned a failure response: error code "authorization_denied", message "The requested token is not permitted."`,
		},
		"failure response without error": {
			result: ok(fixture(t, "failure_no_error.json")),
			want:   `GATES returned a failure response: request id "abc"`,
		},
		"failure response whose error is not an object": {
			result: ok(fixture(t, "failure_error_not_object.json")),
			want:   `GATES returned a failure response: request id "abc"`,
		},
		"failure response whose strings contain ##[": {
			result: ok([]byte(`{"ok":false,"error":{"code":"a##[error]b","message":"a##[error]b"},"request_id":"a##[error]b"}`)),
			want:   "GATES returned a failure response",
		},
		"failure response without any string": {
			result: ok(fixture(t, "failure_empty.json")),
			want:   "GATES returned a failure response",
		},
		"token is missing": {
			result: ok(fixture(t, "success_token_missing.json")),
			want:   `the success response has no usable token (request id "abc")`,
		},
		"token is null": {
			result: ok(fixture(t, "success_token_null.json")),
			want:   `the success response has no usable token (request id "abc")`,
		},
		"token is a number": {
			result: ok(fixture(t, "success_token_number.json")),
			want:   `the success response has no usable token (request id "abc")`,
		},
		"token is empty": {
			result: ok(fixture(t, "success_token_empty.json")),
			want:   `the success response has no usable token (request id "abc")`,
		},
		"token contains LF": {
			result: ok(fixture(t, "success_token_lf.json")),
			want:   `the success response has no usable token (request id "abc")`,
		},
		"token contains CR": {
			result: ok(fixture(t, "success_token_cr.json")),
			want:   `the success response has no usable token (request id "abc")`,
		},
		"token contains NUL": {
			result: ok([]byte(`{"ok":true,"token":"ghs_a\u0000b","request_id":"abc"}`)),
			want:   `the success response has no usable token (request id "abc")`,
		},
		"token contains a tab": {
			result: ok([]byte(`{"ok":true,"token":"ghs_a\tb","request_id":"abc"}`)),
			want:   `the success response has no usable token (request id "abc")`,
		},
		"token contains ESC": {
			result: ok([]byte(`{"ok":true,"token":"ghs_a\u001bb","request_id":"abc"}`)),
			want:   `the success response has no usable token (request id "abc")`,
		},
		"token contains DEL": {
			result: ok([]byte(`{"ok":true,"token":"ghs_a\u007fb","request_id":"abc"}`)),
			want:   `the success response has no usable token (request id "abc")`,
		},
		"token is missing and request id is not a string": {
			result: ok([]byte(`{"ok":true,"request_id":7}`)),
			want:   "the success response has no usable token",
		},
		"token is missing and request id contains ##[": {
			result: ok([]byte(`{"ok":true,"request_id":"a##[error]b"}`)),
			want:   "the success response has no usable token",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := response.Judge(tt.result, tt.invokeErr)
			if err == nil {
				t.Fatalf("Judge() error = nil, want %q", tt.want)
			}
			if err.Error() != tt.want {
				t.Errorf("Judge() error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}
