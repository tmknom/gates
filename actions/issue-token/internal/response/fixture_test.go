package response_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tmknom/gates/actions/issue-token/internal/awsapi"
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
