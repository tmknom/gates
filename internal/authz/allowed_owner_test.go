package authz_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/repository"
)

func TestNewAllowedOwner(t *testing.T) {
	tests := []struct {
		name      string
		ownerType authz.OwnerType
	}{
		{name: "organization owner is accepted", ownerType: authz.OwnerTypeOrganization},
		{name: "user owner is accepted", ownerType: authz.OwnerTypeUser},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := authz.NewAllowedOwner(12345, "Example-Org", tt.ownerType)
			if err != nil {
				t.Fatalf("NewAllowedOwner returned error: %v", err)
			}
			if got.ID() != 12345 {
				t.Errorf("ID() = %d, want 12345", got.ID())
			}
			if got.Name().String() != "Example-Org" {
				t.Errorf("Name() = %q, want %q", got.Name(), "Example-Org")
			}
			if got.Type() != tt.ownerType {
				t.Errorf("Type() = %q, want %q", got.Type(), tt.ownerType)
			}
		})
	}
}

func TestAllowedOwner_IsZero(t *testing.T) {
	owner, err := authz.NewAllowedOwner(12345, "Example-Org", authz.OwnerTypeOrganization)
	if err != nil {
		t.Fatalf("NewAllowedOwner returned error: %v", err)
	}
	if owner.IsZero() {
		t.Error("IsZero() = true for the AllowedOwner created by NewAllowedOwner")
	}
	if !(authz.AllowedOwner{}).IsZero() {
		t.Error("IsZero() = false for the zero AllowedOwner")
	}
}

func TestNewAllowedOwner_Invalid(t *testing.T) {
	tests := []struct {
		name      string
		id        int64
		ownerName string
		ownerType authz.OwnerType
		wantErr   error
	}{
		{name: "zero owner id returns ErrInvalidAllowedOwner", id: 0, ownerName: "example", ownerType: authz.OwnerTypeOrganization, wantErr: authz.ErrInvalidAllowedOwner},
		{name: "negative owner id returns ErrInvalidAllowedOwner", id: -1, ownerName: "example", ownerType: authz.OwnerTypeOrganization, wantErr: authz.ErrInvalidAllowedOwner},
		{name: "malformed owner name returns ErrInvalidAllowedOwner wrapping ErrInvalidOwnerName", id: 1, ownerName: "ex_ample", ownerType: authz.OwnerTypeOrganization, wantErr: repository.ErrInvalidOwnerName},
		{name: "missing owner type returns ErrInvalidAllowedOwner", id: 1, ownerName: "example", ownerType: "", wantErr: authz.ErrInvalidAllowedOwner},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := authz.NewAllowedOwner(tt.id, tt.ownerName, tt.ownerType)
			if !errors.Is(err, authz.ErrInvalidAllowedOwner) {
				t.Errorf("NewAllowedOwner() error = %v, want %v", err, authz.ErrInvalidAllowedOwner)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("NewAllowedOwner() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

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
