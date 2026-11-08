package authz_test

import (
	"errors"
	"testing"
	"time"

	"github.com/tmknom/gates/internal/authz"
)

func TestInstallation_IDAndInstallationOwnerID(t *testing.T) {
	installation, err := authz.NewInstallation(installationID, allowedOwnerID, time.Time{})
	if err != nil {
		t.Fatalf("NewInstallation() error = %v", err)
	}
	if got := installation.ID().Int64(); got != installationID {
		t.Errorf("ID() = %d, want %d", got, installationID)
	}
	if got := installation.InstallationOwnerID().Int64(); got != allowedOwnerID {
		t.Errorf("InstallationOwnerID() = %d, want %d", got, allowedOwnerID)
	}
}

func TestNewInstallation_Invalid(t *testing.T) {
	tests := []struct {
		name                string
		id                  int64
		installationOwnerID int64
	}{
		{name: "zero installation id returns ErrInvalidInstallation", id: 0, installationOwnerID: allowedOwnerID},
		{name: "zero installation owner id returns ErrInvalidInstallation", id: installationID, installationOwnerID: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := authz.NewInstallation(tt.id, tt.installationOwnerID, time.Time{})
			if !errors.Is(err, authz.ErrInvalidInstallation) {
				t.Errorf("NewInstallation() error = %v, want %v", err, authz.ErrInvalidInstallation)
			}
		})
	}
}
