package authz_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/authz"
)

//trace:checks AC-AUTHZ-017
func TestParseRepositoryPolicyEnforcement(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want authz.RepositoryPolicyEnforcement
	}{
		{name: "true is enforced", s: "true", want: authz.RepositoryPolicyEnforced},
		{name: "false is not enforced", s: "false", want: authz.RepositoryPolicyNotEnforced},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := authz.ParseRepositoryPolicyEnforcement(tt.s)
			if err != nil {
				t.Fatalf("ParseRepositoryPolicyEnforcement(%q) returned error: %v", tt.s, err)
			}
			if got != tt.want {
				t.Errorf("ParseRepositoryPolicyEnforcement(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

// 欠落と書き損じをどちらかの値に読むと運用者が決めていない動作になるため（INV-018）、true と false 以外の表記はどれも誤りにする。
//
//trace:checks AC-AUTHZ-017
func TestParseRepositoryPolicyEnforcement_Invalid(t *testing.T) {
	for _, s := range []string{"True", "1", "yes", " true", "true ", ""} {
		t.Run(fmt.Sprintf("%q returns ErrInvalidRepositoryPolicyEnforcement", s), func(t *testing.T) {
			got, err := authz.ParseRepositoryPolicyEnforcement(s)
			if !errors.Is(err, authz.ErrInvalidRepositoryPolicyEnforcement) {
				t.Fatalf("ParseRepositoryPolicyEnforcement(%q) error = %v, want %v", s, err, authz.ErrInvalidRepositoryPolicyEnforcement)
			}
			if !strings.Contains(err.Error(), "must be true or false") {
				t.Errorf("ParseRepositoryPolicyEnforcement(%q) error = %q, want it to contain %q", s, err, "must be true or false")
			}
			if !got.IsZero() {
				t.Errorf("ParseRepositoryPolicyEnforcement(%q) returned a non-zero value with an error", s)
			}
		})
	}
}

func TestParseRepositoryPolicyEnforcement_ErrorDoesNotContainValue(t *testing.T) {
	const marker = "secret-enforcement"
	_, err := authz.ParseRepositoryPolicyEnforcement(marker)
	if err == nil {
		t.Fatal("ParseRepositoryPolicyEnforcement() returned no error")
	}
	if strings.Contains(err.Error(), marker) {
		t.Errorf("error %q contains value %q", err, marker)
	}
}

func TestRepositoryPolicyEnforcement_IsZero(t *testing.T) {
	tests := []struct {
		name        string
		enforcement authz.RepositoryPolicyEnforcement
		want        bool
	}{
		{name: "zero value is zero", enforcement: authz.RepositoryPolicyEnforcement{}, want: true},
		{name: "enforced is not zero", enforcement: authz.RepositoryPolicyEnforced, want: false},
		{name: "not enforced is not zero", enforcement: authz.RepositoryPolicyNotEnforced, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.enforcement.IsZero(); got != tt.want {
				t.Errorf("IsZero() = %v, want %v", got, tt.want)
			}
		})
	}
}
