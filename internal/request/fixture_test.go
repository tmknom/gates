package request_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/repository"
)

// payloadFields は、正当な Payload のフィールドを並び順どおりに持つ。値は JSON の断片である。
var payloadFields = []struct{ name, value string }{
	{name: "version", value: `1`},
	{name: "id_token", value: `"header.payload.signature"`},
	{name: "repositories", value: `["example-org/repo-a","example-org/repo-b"]`},
	{name: "permissions", value: `{"contents":"read"}`},
}

// payloadWith は、正当な Payload の name の値を value に差し替える。name が既知のフィールドでなければ末尾に加える。
func payloadWith(name, value string) string {
	members := make([]string, 0, len(payloadFields)+1)
	replaced := false
	for _, f := range payloadFields {
		if f.name == name {
			members = append(members, member(name, value))
			replaced = true
			continue
		}
		members = append(members, member(f.name, f.value))
	}
	if !replaced {
		members = append(members, member(name, value))
	}
	return "{" + strings.Join(members, ",") + "}"
}

func member(name, value string) string {
	return fmt.Sprintf("%q:%s", name, value)
}

func assertRepositories(t *testing.T, got []repository.FullName, want []string) {
	t.Helper()
	// FullName.Equal は大文字小文字を区別しないため、要求どおりの表記が保たれたかは String() の列で比べる
	names := make([]string, len(got))
	for i, f := range got {
		names[i] = f.String()
	}
	if !slices.Equal(names, want) {
		t.Errorf("Repositories() = %v, want %v", names, want)
	}
}
