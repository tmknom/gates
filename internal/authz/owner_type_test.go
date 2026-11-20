package authz_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/tmknom/gates/internal/authz"
)

func TestParseOwnerType(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want authz.OwnerType
	}{
		{name: "Organization is accepted", s: "Organization", want: authz.OwnerTypeOrganization},
		{name: "User is accepted", s: "User", want: authz.OwnerTypeUser},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := authz.ParseOwnerType(tt.s)
			if err != nil {
				t.Fatalf("ParseOwnerType returned error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ParseOwnerType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseOwnerType_Invalid(t *testing.T) {
	for _, s := range []string{"organization", "Org"} {
		t.Run(fmt.Sprintf("owner type %q returns ErrInvalidAllowedOwner", s), func(t *testing.T) {
			_, err := authz.ParseOwnerType(s)
			if !errors.Is(err, authz.ErrInvalidAllowedOwner) {
				t.Errorf("ParseOwnerType() error = %v, want %v", err, authz.ErrInvalidAllowedOwner)
			}
		})
	}
}
