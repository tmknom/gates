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

// credential は、値を伏せて出力する資格情報と、その値である。
type credential struct {
	name   string
	output any
	value  string
}

// credentials は、GitHub Client が扱う資格情報を返す。Installation Access Token は発行応答からしか作れないため、Issue で読み取る。
func credentials(t *testing.T) []credential {
	t.Helper()
	token, ok := issueBody(t, marshal(t, recordedIssuance())).Token()
	if !ok {
		t.Fatal("Token() was not read")
	}
	return []credential{
		{name: "GitHub App JWT", output: github.NewAppJWT(appJWTValue), value: appJWTValue},
		{name: "Installation Access Token", output: token, value: tokenMarker},
	}
}

func TestCredentials_AreRedactedInOutput(t *testing.T) {
	tests := []struct {
		name   string
		output func(t *testing.T, value any) string
	}{
		{name: "%v does not show the value", output: func(_ *testing.T, value any) string { return fmt.Sprintf("%v", value) }},
		{name: "%#v does not show the value", output: func(_ *testing.T, value any) string { return fmt.Sprintf("%#v", value) }},
		{name: "exported field with %+v does not show the value", output: func(_ *testing.T, value any) string {
			return fmt.Sprintf("%+v", struct{ Credential any }{Credential: value})
		}},
		{name: "slog attribute does not show the value", output: func(_ *testing.T, value any) string {
			var buf bytes.Buffer
			slog.New(slog.NewJSONHandler(&buf, nil)).Info("msg", "credential", value, "holder", struct{ Credential any }{Credential: value})
			return buf.String()
		}},
		{name: "encoding/json does not show the value", output: func(t *testing.T, value any) string {
			b, err := jsonv1.Marshal(struct{ Credential any }{Credential: value})
			if err != nil {
				t.Fatalf("encoding/json Marshal returned error: %v", err)
			}
			return string(b)
		}},
		{name: "encoding/json/v2 does not show the value", output: func(t *testing.T, value any) string {
			b, err := json.Marshal(struct{ Credential any }{Credential: value})
			if err != nil {
				t.Fatalf("encoding/json/v2 Marshal returned error: %v", err)
			}
			return string(b)
		}},
	}
	for _, c := range credentials(t) {
		for _, tt := range tests {
			t.Run(c.name+" "+tt.name, func(t *testing.T) {
				got := tt.output(t, c.output)
				if strings.Contains(got, c.value) {
					t.Errorf("output %q contains the %s", got, c.name)
				}
				if !strings.Contains(got, "[REDACTED]") {
					t.Errorf("output %q does not contain the redacted notation", got)
				}
			})
		}
	}
}

// TestCredentials_UnexportedFieldDoesNotShowValue は、fmt が Format を呼べない非公開フィールドに置かれても、値が出ないことを確かめる。
func TestCredentials_UnexportedFieldDoesNotShowValue(t *testing.T) {
	appJWTHolder := struct{ credential github.AppJWT }{credential: github.NewAppJWT(appJWTValue)}
	token, _ := issueBody(t, marshal(t, recordedIssuance())).Token()
	tokenHolder := struct {
		credential github.InstallationAccessToken
	}{credential: token}

	for _, holder := range []any{appJWTHolder, tokenHolder} {
		for _, verb := range []string{"%v", "%#v"} {
			t.Run(fmt.Sprintf("%T %s does not show the value", holder, verb), func(t *testing.T) {
				got := fmt.Sprintf(verb, holder)
				for _, value := range []string{appJWTValue, tokenMarker} {
					if strings.Contains(got, value) {
						t.Errorf("output %q contains %q", got, value)
					}
				}
			})
		}
	}
}
