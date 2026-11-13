package request_test

import (
	"encoding/json/v2"
	"fmt"
	"log/slog"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/request"
)

const secretIDToken = "secret.jwt.value"

// TestUnverifiedIDToken_IsRedactedInOutput は、credential.Redacted のメソッドが UnverifiedIDToken に昇格していることを確かめる。
// 書式指定や出力先ごとの網羅は credential package のテストが確かめる。
//
//trace:checks AC-19
func TestUnverifiedIDToken_IsRedactedInOutput(t *testing.T) {
	token := validateIDToken(t, secretIDToken)
	holder := struct{ Token request.UnverifiedIDToken }{Token: token}

	tests := []struct {
		name   string
		output func(t *testing.T) string
	}{
		{name: "%v does not show the value", output: func(*testing.T) string { return fmt.Sprintf("%v", token) }},
		// slog の handler は LogValue がなくても MarshalJSON や Format で伏せた表記を出すため、解決した値の種類で LogValue を確かめる
		{name: "slog value resolves to the redacted notation", output: func(t *testing.T) string {
			resolved := slog.AnyValue(token).Resolve()
			if resolved.Kind() != slog.KindString {
				t.Errorf("slog value kind = %v, want %v", resolved.Kind(), slog.KindString)
			}
			return resolved.String()
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

// TestUnverifiedIDToken_UnexportedFieldDoesNotShowValue は、fmt が Format を呼べない非公開フィールドに置かれても、値が出ないことを確かめる。
func TestUnverifiedIDToken_UnexportedFieldDoesNotShowValue(t *testing.T) {
	holder := struct{ token request.UnverifiedIDToken }{token: validateIDToken(t, secretIDToken)}
	for _, verb := range []string{"%v", "%+v", "%#v"} {
		t.Run(verb+" does not show the value", func(t *testing.T) {
			if got := fmt.Sprintf(verb, holder); strings.Contains(got, secretIDToken) {
				t.Errorf("output %q contains the ID token", got)
			}
		})
	}
}

// TestUnverifiedIDToken_ExposeZeroValuePanicsAsZeroValueGuard は、ゼロ値を nil の参照外しではなくゼロ値の検査で止めることを確かめる。
func TestUnverifiedIDToken_ExposeZeroValuePanicsAsZeroValueGuard(t *testing.T) {
	defer func() {
		got := recover()
		if got == nil {
			t.Fatal("Expose did not panic for the zero UnverifiedIDToken")
		}
		if _, ok := got.(runtime.Error); ok {
			t.Errorf("Expose panicked with a runtime error: %v", got)
		}
	}()

	_ = request.UnverifiedIDToken{}.Expose()
}

// validateIDToken は、UnverifiedIDToken を作れる唯一の経路である Validate を通して値を得る。
func validateIDToken(t *testing.T, idToken string) request.UnverifiedIDToken {
	t.Helper()
	token, _, err := request.Validate([]byte(payloadWith("id_token", strconv.Quote(idToken))))
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	return token
}
