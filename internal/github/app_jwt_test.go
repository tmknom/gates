package github_test

import (
	"bytes"
	jsonv1 "encoding/json"
	"encoding/json/v2"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/github"
)

const appJWTValue = "secret.app.jwt"

func TestAppJWT_IsRedactedInOutput(t *testing.T) {
	token := github.NewAppJWT(appJWTValue)
	holder := struct{ Token github.AppJWT }{Token: token}

	tests := []struct {
		name   string
		output func(t *testing.T) string
	}{
		{name: "%v does not show the value", output: func(*testing.T) string { return fmt.Sprintf("%v", token) }},
		{name: "%#v does not show the value", output: func(*testing.T) string { return fmt.Sprintf("%#v", token) }},
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
			if strings.Contains(got, appJWTValue) {
				t.Errorf("output %q contains the GitHub App JWT", got)
			}
			if !strings.Contains(got, "[REDACTED]") {
				t.Errorf("output %q does not contain the redacted notation", got)
			}
		})
	}
}

// TestAppJWT_UnexportedFieldDoesNotShowValue は、fmt が Format を呼べない非公開フィールドに置かれても、値が出ないことを確かめる。
func TestAppJWT_UnexportedFieldDoesNotShowValue(t *testing.T) {
	holder := struct{ token github.AppJWT }{token: github.NewAppJWT(appJWTValue)}
	for _, verb := range []string{"%v", "%#v"} {
		t.Run(verb+" does not show the value", func(t *testing.T) {
			if got := fmt.Sprintf(verb, holder); strings.Contains(got, appJWTValue) {
				t.Errorf("output %q contains the GitHub App JWT", got)
			}
		})
	}
}
