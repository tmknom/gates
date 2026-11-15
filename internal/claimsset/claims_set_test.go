package claimsset_test

import (
	"errors"
	"testing"

	"github.com/tmknom/gates/internal/claimsset"
)

func TestNewSignatureVerified(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    string
	}{
		{
			name:    "object keeps every claim including unknown ones",
			payload: `{"sub":"repo:o@1/r@2:ref:refs/heads/main","repo_property_tier":"gold","unknown":{"nested":[1,true,null]}}`,
			want:    `{"sub":"repo:o@1/r@2:ref:refs/heads/main","repo_property_tier":"gold","unknown":{"nested":[1,true,null]}}`,
		},
		{
			name:    "whitespace between tokens is removed",
			payload: " {\n\t\"a\" : [ 1 , 2 ] } ",
			want:    `{"a":[1,2]}`,
		},
		{
			name:    "control characters stay escaped",
			payload: `{"head_ref":"a\u0001b\nc\u000Ad"}`,
			want:    `{"head_ref":"a\u0001b\nc\nd"}`,
		},
		{
			name:    "line and paragraph separators are escaped",
			payload: "{\"actor\":\"a" + string(rune(0x2028)) + "b" + string(rune(0x2029)) + "c\"}",
			want:    `{"actor":"a\u2028b\u2029c"}`,
		},
		{
			name:    "escapes that are not needed are rewritten",
			payload: `{"a":"\/b"}`,
			want:    `{"a":"/b"}`,
		},
		{
			name:    "number notation is kept",
			payload: `{"exp":1.50e2,"big":12345678901234567890123}`,
			want:    `{"exp":1.50e2,"big":12345678901234567890123}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := claimsset.NewSignatureVerified([]byte(tt.payload))
			if err != nil {
				t.Fatalf("NewSignatureVerified returned error: %v", err)
			}
			b, err := got.MarshalJSON()
			if err != nil {
				t.Fatalf("MarshalJSON returned error: %v", err)
			}
			if string(b) != tt.want {
				t.Errorf("MarshalJSON = %s, want %s", b, tt.want)
			}
		})
	}
}

// OIDC Verifier は、同じ Payload を Claim の検証にも使う。
func TestNewSignatureVerified_DoesNotModifyPayload(t *testing.T) {
	const original = ` {"a" : "\/b"} `
	payload := []byte(original)
	if _, err := claimsset.NewSignatureVerified(payload); err != nil {
		t.Fatalf("NewSignatureVerified returned error: %v", err)
	}
	if string(payload) != original {
		t.Errorf("payload = %s, want %s unchanged", payload, original)
	}
}

func TestNewSignatureVerified_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{name: "malformed JSON returns ErrInvalidClaimsSet", payload: `{"a":`},
		{name: "duplicate claim names return ErrInvalidClaimsSet", payload: `{"a":1,"a":2}`},
		{name: "invalid UTF-8 returns ErrInvalidClaimsSet", payload: "{\"a\":\"\xff\"}"},
		{name: "array returns ErrInvalidClaimsSet", payload: `[{"a":1}]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := claimsset.NewSignatureVerified([]byte(tt.payload))
			if !errors.Is(err, claimsset.ErrInvalidClaimsSet) {
				t.Fatalf("NewSignatureVerified error = %v, want ErrInvalidClaimsSet", err)
			}
			if !got.IsZero() {
				t.Errorf("NewSignatureVerified returned a non-zero value with an error")
			}
		})
	}
}

func TestSignatureVerified_IsZero(t *testing.T) {
	if !(claimsset.SignatureVerified{}).IsZero() {
		t.Errorf("zero value IsZero() = false, want true")
	}
	got, err := claimsset.NewSignatureVerified([]byte(`{}`))
	if err != nil {
		t.Fatalf("NewSignatureVerified returned error: %v", err)
	}
	if got.IsZero() {
		t.Errorf("empty object IsZero() = true, want false")
	}
}
