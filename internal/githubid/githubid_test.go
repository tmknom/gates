package githubid_test

import (
	"errors"
	"testing"

	"github.com/tmknom/gates/internal/githubid"
)

func TestNewIDs(t *testing.T) {
	constructors := []struct {
		name string
		new  func(int64) (int64, error)
	}{
		{name: "owner id", new: func(v int64) (int64, error) {
			id, err := githubid.NewOwnerID(v)
			return id.Int64(), err
		}},
		{name: "repository id", new: func(v int64) (int64, error) {
			id, err := githubid.NewRepositoryID(v)
			return id.Int64(), err
		}},
		{name: "installation id", new: func(v int64) (int64, error) {
			id, err := githubid.NewInstallationID(v)
			return id.Int64(), err
		}},
		{name: "run id", new: func(v int64) (int64, error) {
			id, err := githubid.NewRunID(v)
			return id.Int64(), err
		}},
		{name: "run attempt", new: func(v int64) (int64, error) {
			attempt, err := githubid.NewRunAttempt(v)
			return attempt.Int64(), err
		}},
	}
	tests := []struct {
		name    string
		value   int64
		wantErr bool
	}{
		{name: "one is accepted", value: 1},
		{name: "zero returns ErrInvalidID", value: 0, wantErr: true},
		{name: "negative returns ErrInvalidID", value: -1, wantErr: true},
	}
	for _, c := range constructors {
		for _, tt := range tests {
			t.Run(c.name+" "+tt.name, func(t *testing.T) {
				got, err := c.new(tt.value)
				if tt.wantErr {
					if !errors.Is(err, githubid.ErrInvalidID) {
						t.Errorf("error = %v, want %v", err, githubid.ErrInvalidID)
					}
					return
				}
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != tt.value {
					t.Errorf("Int64() = %d, want %d", got, tt.value)
				}
			})
		}
	}
}

func TestOwnerID_IsZero(t *testing.T) {
	id, err := githubid.NewOwnerID(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id.IsZero() {
		t.Error("IsZero() = true for a created owner id, want false")
	}
	if !(githubid.OwnerID{}).IsZero() {
		t.Error("IsZero() = false for the zero owner id, want true")
	}
}

func TestInstallationID_IsZero(t *testing.T) {
	id, err := githubid.NewInstallationID(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id.IsZero() {
		t.Error("IsZero() = true for a created installation id, want false")
	}
	if !(githubid.InstallationID{}).IsZero() {
		t.Error("IsZero() = false for the zero installation id, want true")
	}
}
