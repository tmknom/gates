package authz_test

import (
	"errors"
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
