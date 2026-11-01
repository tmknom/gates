package authz_test

import (
	"errors"
	"testing"
	"time"

	"github.com/tmknom/gates/internal/authz"
)

func TestNewInstallation_Invalid(t *testing.T) {
	tests := []struct {
		name                string
		id                  int64
		installationOwnerID int64
	}{
		{name: "zero installation id returns ErrInvalidInstallation", id: 0, installationOwnerID: allowedOwnerID},
		{name: "negative installation id returns ErrInvalidInstallation", id: -1, installationOwnerID: allowedOwnerID},
		{name: "zero installation owner id returns ErrInvalidInstallation", id: installationID, installationOwnerID: 0},
		{name: "negative installation owner id returns ErrInvalidInstallation", id: installationID, installationOwnerID: -1},
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
