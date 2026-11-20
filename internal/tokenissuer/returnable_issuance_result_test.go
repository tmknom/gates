package tokenissuer

import (
	"bytes"
	jsonv1 "encoding/json"
	"encoding/json/v2"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

//trace:checks AC-QUALITY-002
func TestReturnableIssuanceResult_OutputDoesNotContainToken(t *testing.T) {
	result := returnableResult(t)
	tests := []struct {
		name   string
		output func() string
	}{
		{name: "fmt %v does not contain token", output: func() string { return fmt.Sprintf("%v", result) }},
		{name: "fmt %+v does not contain token", output: func() string { return fmt.Sprintf("%+v", result) }},
		{name: "fmt %#v does not contain token", output: func() string { return fmt.Sprintf("%#v", result) }},
		{name: "slog does not contain token", output: func() string {
			var buf bytes.Buffer
			slog.New(slog.NewJSONHandler(&buf, nil)).Info("issued", "returnable_issuance_result", result)
			return buf.String()
		}},
		// encoding/json/v2 は公開フィールドのない struct の Marshal を error にするため、出力と error の文言の両方を調べる
		{name: "encoding/json does not contain token", output: func() string {
			b, err := jsonv1.Marshal(result) //nolint:staticcheck // 公開フィールドを加えてもトークンが出ないことを確かめるため、出力するフィールドがなくても Marshal する
			return string(b) + fmt.Sprint(err)
		}},
		{name: "encoding/json/v2 does not contain token", output: func() string {
			b, err := json.Marshal(result)
			return string(b) + fmt.Sprint(err)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.output(); strings.Contains(got, tokenMarker) {
				t.Errorf("output %q contains the token", got)
			}
		})
	}
}

func TestReturnableIssuanceResult_EffectiveRepositoriesCopiesValue(t *testing.T) {
	result := returnableResult(t)
	repositories := result.EffectiveRepositories()
	repositories[0] = repositories[1]

	if got := result.EffectiveRepositories()[0].RepositoryID(); got != repositoryIDA {
		t.Errorf("EffectiveRepositories()[0].RepositoryID() = %d after modifying the returned slice, want %d", got, repositoryIDA)
	}
}

func returnableResult(t *testing.T) ReturnableIssuanceResult {
	t.Helper()
	got, err := verifyResult(t, issue(t, marshal(t, recordedIssuance())), contentsRead)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	return got
}
