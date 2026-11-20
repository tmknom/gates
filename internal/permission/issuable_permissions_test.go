package permission_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/permission"
)

func TestNewIssuablePermissions(t *testing.T) {
	tests := []struct {
		name      string
		notations []string
	}{
		{name: "single entry is accepted", notations: []string{"contents:read"}},
		{name: "two levels of the same permission name are accepted", notations: []string{"contents:read", "contents:write"}},
		{name: "implied permission entry is accepted", notations: []string{"metadata:read"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := permission.NewIssuablePermissions(tt.notations)
			if err != nil {
				t.Fatalf("NewIssuablePermissions(%v) returned error: %v", tt.notations, err)
			}
			if got.IsZero() {
				t.Errorf("NewIssuablePermissions(%v).IsZero() = true, want false", tt.notations)
			}
		})
	}
}

//trace:checks AC-AUTHZ-010
func TestNewIssuablePermissions_Invalid(t *testing.T) {
	tests := []struct {
		name      string
		notations []string
		// condition は、満たさなかった条件と組の位置として文言に書かれる語である
		condition string
	}{
		{name: "empty notations return ErrInvalidIssuablePermissions", notations: []string{}, condition: "must have at least one permission entry"},
		{name: "malformed entry returns ErrInvalidIssuablePermissions", notations: []string{"contents:read", "contents"}, condition: "entries[1] must be a permission name and a level joined by ':'"},
		{name: "entry with surrounding space returns ErrInvalidIssuablePermissions", notations: []string{"contents:read", " issues:read"}, condition: "entries[1] permission name must be"},
		{name: "duplicate entry returns ErrInvalidIssuablePermissions", notations: []string{"contents:read", "contents:read"}, condition: "entries[1] is a duplicate"},
		{name: "organization permission returns ErrInvalidIssuablePermissions", notations: []string{"contents:read", "members:read"}, condition: "entries[1] is not in allowed permissions"},
		{name: "permission name not in enumeration returns ErrInvalidIssuablePermissions", notations: []string{"nonexistent_permission:read"}, condition: "entries[0] is not in allowed permissions"},
		{name: "level not allowed for permission name returns ErrInvalidIssuablePermissions", notations: []string{"contents:admin"}, condition: "entries[0] is not in allowed permissions"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := permission.NewIssuablePermissions(tt.notations)
			if !errors.Is(err, permission.ErrInvalidIssuablePermissions) {
				t.Fatalf("NewIssuablePermissions(%v) error = %v, want %v", tt.notations, err, permission.ErrInvalidIssuablePermissions)
			}
			if !strings.Contains(err.Error(), tt.condition) {
				t.Errorf("NewIssuablePermissions(%v) error = %q, want it to contain %q", tt.notations, err, tt.condition)
			}
		})
	}
}

func TestNewIssuablePermissions_ReportsEveryInvalidEntry(t *testing.T) {
	notations := []string{"members:read", "contents:read", "contents", "contents:read", "metadata:write"}
	_, err := permission.NewIssuablePermissions(notations)
	if err == nil {
		t.Fatalf("NewIssuablePermissions(%v) returned no error", notations)
	}
	for _, want := range []string{"entries[0] is not in allowed permissions", "entries[2] must be", "entries[3] is a duplicate", "entries[4] is not in allowed permissions"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to contain %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "entries[1]") {
		t.Errorf("error = %q reports the valid entries[1]", err)
	}
	// 固定設定の読み込みは環境変数ごとの誤りを改行で並べるため、組ごとの誤りは 1 行に収める
	if strings.Contains(err.Error(), "\n") {
		t.Errorf("error = %q spans multiple lines", err)
	}
}

func TestNewIssuablePermissions_ErrorDoesNotContainInput(t *testing.T) {
	tests := []struct {
		name      string
		notations []string
	}{
		{name: "entry outside allowed permissions is not in error message", notations: []string{"secret_value:read"}},
		{name: "duplicate entry is not in error message", notations: []string{"contents:read", "contents:read"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := permission.NewIssuablePermissions(tt.notations)
			if err == nil {
				t.Fatalf("NewIssuablePermissions(%v) returned no error", tt.notations)
			}
			if msg := err.Error(); strings.Contains(msg, "secret") || strings.Contains(msg, "contents") {
				t.Errorf("error message %q contains the input", msg)
			}
		})
	}
}

func TestIssuablePermissions_ZeroValueIsZero(t *testing.T) {
	if !(permission.IssuablePermissions{}).IsZero() {
		t.Errorf("IssuablePermissions{}.IsZero() = false, want true")
	}
}

//trace:checks AC-AUTHZ-009
func TestIssuablePermissions_Contains(t *testing.T) {
	tests := []struct {
		name                string
		issuablePermissions []string
		levels              map[string]string
		contains            bool
	}{
		{name: "all entries in issuable permissions return true", issuablePermissions: []string{"contents:read", "issues:write"}, levels: map[string]string{"contents": "read", "issues": "write"}, contains: true},
		{name: "one of two levels of the same permission name returns true", issuablePermissions: []string{"contents:read", "contents:write"}, levels: map[string]string{"contents": "read"}, contains: true},
		{name: "entry outside issuable permissions mixed with entry inside returns false", issuablePermissions: []string{"contents:read"}, levels: map[string]string{"contents": "read", "issues": "read"}, contains: false},
		{name: "lower level than issuable level returns false", issuablePermissions: []string{"contents:write"}, levels: map[string]string{"contents": "read"}, contains: false},
		{name: "implied permission entry outside issuable permissions returns false", issuablePermissions: []string{"contents:read"}, levels: map[string]string{"contents": "read", "metadata": "read"}, contains: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issuablePermissions := mustNewIssuablePermissions(t, tt.issuablePermissions)
			if got := issuablePermissions.Contains(mustNewPermissions(t, tt.levels)); got != tt.contains {
				t.Errorf("%v.Contains(%v) = %v, want %v", tt.issuablePermissions, tt.levels, got, tt.contains)
			}
		})
	}
}

func TestIssuablePermissions_ContainsZeroValueIsFalse(t *testing.T) {
	t.Run("zero value permissions return false", func(t *testing.T) {
		if mustNewIssuablePermissions(t, []string{"contents:read"}).Contains(permission.Permissions{}) {
			t.Errorf("Contains(Permissions{}) = true, want false")
		}
	})
	t.Run("zero value issuable permissions return false", func(t *testing.T) {
		if (permission.IssuablePermissions{}).Contains(mustNewPermissions(t, map[string]string{"contents": "read"})) {
			t.Errorf("IssuablePermissions{}.Contains({contents:read}) = true, want false")
		}
	})
}

func mustNewIssuablePermissions(t *testing.T, notations []string) permission.IssuablePermissions {
	t.Helper()
	issuablePermissions, err := permission.NewIssuablePermissions(notations)
	if err != nil {
		t.Fatalf("NewIssuablePermissions(%v) returned error: %v", notations, err)
	}
	return issuablePermissions
}
