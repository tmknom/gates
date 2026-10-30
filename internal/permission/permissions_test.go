package permission_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/permission"
)

func TestNewPermissions(t *testing.T) {
	tests := []struct {
		name   string
		levels map[string]string
	}{
		{name: "single permission entry is accepted", levels: map[string]string{"contents": "read"}},
		{name: "permission entry outside allowed permissions is accepted", levels: map[string]string{"contents": "read", "metadata": "read"}},
		{name: "permission name with underscore is accepted", levels: map[string]string{"pull_requests": "write"}},
		{name: "permission name and level of 64 characters are accepted", levels: map[string]string{strings.Repeat("a", 64): strings.Repeat("z", 64)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := permission.NewPermissions(tt.levels); err != nil {
				t.Errorf("NewPermissions(%v) returned error: %v", tt.levels, err)
			}
		})
	}
}

func TestNewPermissions_CopiesInput(t *testing.T) {
	levels := map[string]string{"contents": "read"}
	got, err := permission.NewPermissions(levels)
	if err != nil {
		t.Fatalf("NewPermissions returned error: %v", err)
	}
	levels["contents"] = "write"

	want := mustNewPermissions(t, map[string]string{"contents": "read"})
	if !got.Equal(want) {
		t.Errorf("Permissions changed after the input map was modified")
	}
}

func TestNewPermissions_Invalid(t *testing.T) {
	tests := []struct {
		name   string
		levels map[string]string
	}{
		{name: "empty map returns ErrMalformedPermissions", levels: map[string]string{}},
		{name: "empty permission name returns ErrMalformedPermissions", levels: map[string]string{"": "read"}},
		{name: "permission name of 65 characters returns ErrMalformedPermissions", levels: map[string]string{strings.Repeat("a", 65): "read"}},
		{name: "uppercase permission name returns ErrMalformedPermissions", levels: map[string]string{"Contents": "read"}},
		{name: "permission name with newline returns ErrMalformedPermissions", levels: map[string]string{"contents\n": "read"}},
		{name: "permission name with character just after z returns ErrMalformedPermissions", levels: map[string]string{"contents{": "read"}},
		{name: "permission name with character just before a returns ErrMalformedPermissions", levels: map[string]string{"contents`": "read"}},
		{name: "permission level with hyphen returns ErrMalformedPermissions", levels: map[string]string{"contents": "read-only"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := permission.NewPermissions(tt.levels)
			if !errors.Is(err, permission.ErrMalformedPermissions) {
				t.Errorf("NewPermissions(%v) error = %v, want %v", tt.levels, err, permission.ErrMalformedPermissions)
			}
		})
	}
}

func TestNewPermissions_ErrorDoesNotContainInput(t *testing.T) {
	tests := []struct {
		name   string
		levels map[string]string
	}{
		{name: "permission name is not in error message", levels: map[string]string{"secret-value": "read"}},
		{name: "permission level is not in error message", levels: map[string]string{"contents": "secret-value"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := permission.NewPermissions(tt.levels)
			if err == nil {
				t.Fatalf("NewPermissions(%v) returned no error", tt.levels)
			}
			if strings.Contains(err.Error(), "secret") {
				t.Errorf("error message %q contains the input", err.Error())
			}
		})
	}
}

func TestPermissions_Equal(t *testing.T) {
	tests := []struct {
		name  string
		a     map[string]string
		b     map[string]string
		equal bool
	}{
		{name: "same permission entries are equal", a: map[string]string{"contents": "read", "metadata": "read"}, b: map[string]string{"metadata": "read", "contents": "read"}, equal: true},
		{name: "different levels are not equal", a: map[string]string{"contents": "read"}, b: map[string]string{"contents": "write"}, equal: false},
		{name: "different names are not equal", a: map[string]string{"contents": "read"}, b: map[string]string{"issues": "read"}, equal: false},
		{name: "additional permission entry is not equal", a: map[string]string{"contents": "read"}, b: map[string]string{"contents": "read", "metadata": "read"}, equal: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := mustNewPermissions(t, tt.a)
			b := mustNewPermissions(t, tt.b)
			if got := a.Equal(b); got != tt.equal {
				t.Errorf("%v.Equal(%v) = %v, want %v", tt.a, tt.b, got, tt.equal)
			}
		})
	}
}

func TestAllowedPermissions(t *testing.T) {
	want := mustNewPermissions(t, map[string]string{"contents": "read"})
	if !permission.AllowedPermissions().Equal(want) {
		t.Errorf("AllowedPermissions() is not equal to {contents:read}")
	}
}

func mustNewPermissions(t *testing.T, levels map[string]string) permission.Permissions {
	t.Helper()
	p, err := permission.NewPermissions(levels)
	if err != nil {
		t.Fatalf("NewPermissions(%v) returned error: %v", levels, err)
	}
	return p
}
