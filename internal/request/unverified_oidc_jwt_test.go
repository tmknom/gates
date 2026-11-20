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

const secretOIDCJWT = "secret.jwt.value"

// TestUnverifiedOIDCJWT_IsRedactedInOutput は、credential.Redacted のメソッドが UnverifiedOIDCJWT に昇格していることを確かめる。
// 書式指定や出力先ごとの網羅は credential package のテストが確かめる。
//
//trace:checks AC-QUALITY-002
func TestUnverifiedOIDCJWT_IsRedactedInOutput(t *testing.T) {
	token := validateOIDCJWT(t, secretOIDCJWT)
	holder := struct{ Token request.UnverifiedOIDCJWT }{Token: token}

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
			if strings.Contains(got, secretOIDCJWT) {
				t.Errorf("output %q contains the OIDC JWT", got)
			}
			if !strings.Contains(got, "[REDACTED]") {
				t.Errorf("output %q does not contain the redacted notation", got)
			}
		})
	}
}

// TestUnverifiedOIDCJWT_UnexportedFieldDoesNotShowValue は、fmt が Format を呼べない非公開フィールドに置かれても、値が出ないことを確かめる。
func TestUnverifiedOIDCJWT_UnexportedFieldDoesNotShowValue(t *testing.T) {
	holder := struct{ token request.UnverifiedOIDCJWT }{token: validateOIDCJWT(t, secretOIDCJWT)}
	for _, verb := range []string{"%v", "%+v", "%#v"} {
		t.Run(verb+" does not show the value", func(t *testing.T) {
			if got := fmt.Sprintf(verb, holder); strings.Contains(got, secretOIDCJWT) {
				t.Errorf("output %q contains the OIDC JWT", got)
			}
		})
	}
}

// TestUnverifiedOIDCJWT_ExposeZeroValuePanicsAsZeroValueGuard は、ゼロ値を nil の参照外しではなくゼロ値の検査で止めることを確かめる。
func TestUnverifiedOIDCJWT_ExposeZeroValuePanicsAsZeroValueGuard(t *testing.T) {
	defer func() {
		got := recover()
		if got == nil {
			t.Fatal("Expose did not panic for the zero UnverifiedOIDCJWT")
		}
		if _, ok := got.(runtime.Error); ok {
			t.Errorf("Expose panicked with a runtime error: %v", got)
		}
	}()

	_ = request.UnverifiedOIDCJWT{}.Expose()
}

// validateOIDCJWT は、UnverifiedOIDCJWT を作れる唯一の経路である Validate を通して値を得る。
func validateOIDCJWT(t *testing.T, oidcJWT string) request.UnverifiedOIDCJWT {
	t.Helper()
	token, _, err := request.Validate([]byte(payloadWith("id_token", strconv.Quote(oidcJWT))))
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	return token
}
