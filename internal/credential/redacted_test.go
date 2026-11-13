package credential_test

import (
	jsonv1 "encoding/json"
	"encoding/json/v2"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/credential"
)

const secretValue = "secret.credential.value"

// sample は Redacted を埋め込む。値をポインタで持たず、Redacted のメソッドが呼ばれなければ値がそのまま出るようにする。
type sample struct {
	credential.Redacted
	Value string
}

// resolveLogValue は、slog が attribute の値として解決した文字列を返す。
// slog の handler は LogValue がなくても MarshalJSON や Format で伏せた表記を出すため、handler の出力ではなく解決した値の種類で LogValue を確かめる。
func resolveLogValue(t *testing.T, value any) string {
	t.Helper()
	resolved := slog.AnyValue(value).Resolve()
	if resolved.Kind() != slog.KindString {
		t.Errorf("slog value kind = %v, want %v", resolved.Kind(), slog.KindString)
	}
	return resolved.String()
}

//trace:checks AC-19
func TestRedacted_HidesValueInOutput(t *testing.T) {
	value := sample{Value: secretValue}
	holder := struct{ Credential sample }{Credential: value}

	tests := []struct {
		name   string
		output func(t *testing.T) string
	}{
		{name: "%v shows the redacted notation", output: func(*testing.T) string { return fmt.Sprintf("%v", value) }},
		{name: "%#v shows the redacted notation", output: func(*testing.T) string { return fmt.Sprintf("%#v", value) }},
		{name: "exported field with %+v shows the redacted notation", output: func(*testing.T) string { return fmt.Sprintf("%+v", holder) }},
		{name: "slog value resolves to the redacted notation", output: func(t *testing.T) string { return resolveLogValue(t, value) }},
		{name: "encoding/json shows the redacted notation", output: func(t *testing.T) string {
			b, err := jsonv1.Marshal(holder)
			if err != nil {
				t.Fatalf("encoding/json Marshal returned error: %v", err)
			}
			return string(b)
		}},
		{name: "encoding/json/v2 shows the redacted notation", output: func(t *testing.T) string {
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
			if strings.Contains(got, secretValue) {
				t.Errorf("output %q contains the value", got)
			}
			if !strings.Contains(got, "[REDACTED]") {
				t.Errorf("output %q does not contain the redacted notation", got)
			}
		})
	}
}
