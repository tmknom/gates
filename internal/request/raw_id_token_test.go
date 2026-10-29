package request_test

import (
	"bytes"
	jsonv1 "encoding/json"
	"encoding/json/v2"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/request"
)

const secretIDToken = "secret.jwt.value"

func TestRawIDToken_IsRedactedInOutput(t *testing.T) {
	token := validateIDToken(t, secretIDToken)
	holder := struct{ Token request.RawIDToken }{Token: token}

	tests := []struct {
		name   string
		output func(t *testing.T) string
	}{
		{name: "%v does not show the value", output: func(*testing.T) string { return fmt.Sprintf("%v", token) }},
		{name: "%+v does not show the value", output: func(*testing.T) string { return fmt.Sprintf("%+v", token) }},
		{name: "%#v does not show the value", output: func(*testing.T) string { return fmt.Sprintf("%#v", token) }},
		{name: "%s does not show the value", output: func(*testing.T) string { return fmt.Sprintf("%s", token) }},
		{name: "%q does not show the value", output: func(*testing.T) string { return fmt.Sprintf("%q", token) }},
		{name: "pointer with %v does not show the value", output: func(*testing.T) string { return fmt.Sprintf("%v", &token) }},
		{name: "exported field with %+v does not show the value", output: func(*testing.T) string { return fmt.Sprintf("%+v", holder) }},
		{name: "slog attribute does not show the value", output: func(*testing.T) string {
			var buf bytes.Buffer
			slog.New(slog.NewJSONHandler(&buf, nil)).Info("msg", "token", token, "holder", holder)
			return buf.String()
		}},
		{name: "encoding/json does not show the value", output: func(t *testing.T) string {
			b, err := jsonv1.Marshal(holder)
			if err != nil {
				t.Fatalf("encoding/json Marshal returned error: %v", err)
			}
			return string(b)
		}},
		{name: "encoding/json/v2 does not show the value", output: func(t *testing.T) string {
			b, err := json.Marshal(holder)
			if err != nil {
				t.Fatalf("encoding/json/v2 Marshal returned error: %v", err)
			}
			return string(b)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.output(t)
			if strings.Contains(got, secretIDToken) {
				t.Errorf("output %q contains the ID token", got)
			}
			if !strings.Contains(got, "[REDACTED]") {
				t.Errorf("output %q does not contain the redacted notation", got)
			}
		})
	}
}

// TestRawIDToken_UnexportedFieldDoesNotShowValue は、fmt が Format を呼べない非公開フィールドに置かれても、値が出ないことを確かめる。
func TestRawIDToken_UnexportedFieldDoesNotShowValue(t *testing.T) {
	holder := struct{ token request.RawIDToken }{token: validateIDToken(t, secretIDToken)}
	for _, verb := range []string{"%v", "%+v", "%#v"} {
		t.Run(verb+" does not show the value", func(t *testing.T) {
			if got := fmt.Sprintf(verb, holder); strings.Contains(got, secretIDToken) {
				t.Errorf("output %q contains the ID token", got)
			}
		})
	}
}

// validateIDToken は、RawIDToken を作れる唯一の経路である Validate を通して値を得る。
func validateIDToken(t *testing.T, idToken string) request.RawIDToken {
	t.Helper()
	token, _, err := request.Validate([]byte(payloadWith("id_token", strconv.Quote(idToken))))
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	return token
}
