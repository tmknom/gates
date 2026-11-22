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

const wantMalformedDecimal = "must be a positive decimal integer without a sign or leading zeros that fits in int64"

// 表記の規則は ParseOwnerID と共有するため、ParseRepositoryID で表記ごとに確かめる
func TestParseRepositoryID(t *testing.T) {
	accepted := []struct {
		name string
		s    string
		want int64
	}{
		{name: "one is accepted", s: "1", want: 1},
		{name: "largest int64 is accepted", s: "9223372036854775807", want: 9223372036854775807},
	}
	for _, tt := range accepted {
		t.Run(tt.name, func(t *testing.T) {
			got, err := githubid.ParseRepositoryID(tt.s)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Int64() != tt.want {
				t.Errorf("Int64() = %d, want %d", got.Int64(), tt.want)
			}
		})
	}
	rejected := []struct {
		name string
		s    string
	}{
		{name: "empty", s: ""},
		{name: "zero", s: "0"},
		{name: "negative", s: "-1"},
		{name: "plus sign", s: "+1"},
		{name: "leading zero", s: "01"},
		{name: "leading space", s: " 1"},
		{name: "trailing space", s: "1 "},
		{name: "fraction", s: "1.0"},
		{name: "exponent", s: "1e3"},
		{name: "hexadecimal", s: "0x10"},
		{name: "underscore", s: "1_000"},
		{name: "non-digit", s: "12a"},
		{name: "int64 overflow", s: "9223372036854775808"},
	}
	for _, tt := range rejected {
		t.Run(tt.name+" is rejected without the value", func(t *testing.T) {
			_, err := githubid.ParseRepositoryID(tt.s)
			if err == nil || err.Error() != wantMalformedDecimal {
				t.Errorf("error = %v, want %q", err, wantMalformedDecimal)
			}
		})
	}
}

func TestParseOwnerID(t *testing.T) {
	t.Run("decimal is accepted", func(t *testing.T) {
		got, err := githubid.ParseOwnerID("12345")
		if err != nil || got.Int64() != 12345 {
			t.Errorf("ParseOwnerID = %d, %v, want 12345", got.Int64(), err)
		}
	})
	t.Run("leading zero is rejected", func(t *testing.T) {
		if _, err := githubid.ParseOwnerID("01"); err == nil || err.Error() != wantMalformedDecimal {
			t.Errorf("error = %v, want %q", err, wantMalformedDecimal)
		}
	})
}
