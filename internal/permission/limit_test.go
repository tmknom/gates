package permission_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/permission"
)

func TestNewLimit(t *testing.T) {
	tests := []struct {
		name    string
		entries []string
	}{
		{name: "single entry is accepted", entries: []string{"contents", "read"}},
		{name: "entries of different permission names are accepted", entries: []string{"contents", "write", "repository_projects", "admin"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := permission.NewLimit(limitEntries(tt.entries...)); err != nil {
				t.Errorf("NewLimit(%v) returned error: %v", tt.entries, err)
			}
		})
	}
}

//trace:checks AC-AUTHZ-013
func TestNewLimit_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		entries []string
		want    string
	}{
		{name: "no entries return ErrInvalidLimit", entries: nil, want: "invalid permission limit: must have at least one permission entry"},
		{name: "malformed permission name returns ErrInvalidLimit", entries: []string{"contents", "read", "Issues", "read"}, want: "invalid permission limit: entries[1] permission name must be 1 to 64 lowercase ASCII letters or '_'"},
		{name: "uppercase level returns ErrInvalidLimit", entries: []string{"contents", "READ"}, want: "invalid permission limit: entries[0] permission level must be 1 to 64 lowercase ASCII letters or '_'"},
		{name: "duplicate permission name returns ErrInvalidLimit", entries: []string{"contents", "read", "contents", "write"}, want: "invalid permission limit: entries[1] is a duplicate permission name"},
		{name: "organization permission returns ErrInvalidLimit", entries: []string{"members", "read"}, want: "invalid permission limit: entries[0] is not in allowed permissions"},
		{name: "level not allowed for permission name returns ErrInvalidLimit", entries: []string{"contents", "admin"}, want: "invalid permission limit: entries[0] is not in allowed permissions"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := permission.NewLimit(limitEntries(tt.entries...))
			if !errors.Is(err, permission.ErrInvalidLimit) || err.Error() != tt.want {
				t.Errorf("NewLimit(%v) error = %v, want %q", tt.entries, err, tt.want)
			}
		})
	}
}

func TestNewLimit_ReportsEveryInvalidEntry(t *testing.T) {
	entries := []string{"members", "read", "contents", "read", "contents", "write", "issues", "Read"}
	_, err := permission.NewLimit(limitEntries(entries...))
	const want = "invalid permission limit: entries[0] is not in allowed permissions; entries[2] is a duplicate permission name; " +
		"entries[3] permission level must be 1 to 64 lowercase ASCII letters or '_'"
	if err == nil || err.Error() != want {
		t.Errorf("NewLimit(%v) error = %v, want %q", entries, err, want)
	}
}

func TestNewLimit_ErrorDoesNotContainInput(t *testing.T) {
	_, err := permission.NewLimit(limitEntries("secret_name", "read", "contents", "secret_level"))
	if err == nil {
		t.Fatal("NewLimit returned no error")
	}
	if strings.Contains(err.Error(), "secret") {
		t.Errorf("error message %q contains the input", err)
	}
}

//trace:checks AC-AUTHZ-011
func TestLimit_Contains(t *testing.T) {
	tests := []struct {
		name     string
		limit    []string
		levels   map[string]string
		contains bool
	}{
		{name: "same level returns true", limit: []string{"contents", "write"}, levels: map[string]string{"contents": "write"}, contains: true},
		{name: "read under write returns true", limit: []string{"contents", "write"}, levels: map[string]string{"contents": "read"}, contains: true},
		{name: "higher level returns false", limit: []string{"contents", "read"}, levels: map[string]string{"contents": "write"}, contains: false},
		{name: "permission name outside the limit returns false", limit: []string{"contents", "write"}, levels: map[string]string{"issues": "read"}, contains: false},
		{name: "one of two entries outside the limit returns false", limit: []string{"contents", "write"}, levels: map[string]string{"contents": "read", "issues": "read"}, contains: false},
		{name: "implied permission entry outside the limit returns false", limit: []string{"contents", "write"}, levels: map[string]string{"contents": "read", "metadata": "read"}, contains: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit := mustNewLimit(t, tt.limit...)
			if got := limit.Contains(mustNewPermissions(t, tt.levels)); got != tt.contains {
				t.Errorf("Limit(%v).Contains(%v) = %v, want %v", tt.limit, tt.levels, got, tt.contains)
			}
		})
	}
}

func TestLimit_ContainsZeroValueIsFalse(t *testing.T) {
	t.Run("zero value limit returns false", func(t *testing.T) {
		if (permission.Limit{}).Contains(mustNewPermissions(t, map[string]string{"contents": "read"})) {
			t.Errorf("Limit{}.Contains({contents:read}) = true, want false")
		}
	})
}

func mustNewLimit(t *testing.T, entries ...string) permission.Limit {
	t.Helper()
	limit, err := permission.NewLimit(limitEntries(entries...))
	if err != nil {
		t.Fatalf("NewLimit(%v) returned error: %v", entries, err)
	}
	return limit
}

// limitEntries は、権限名と水準を交互に並べた列を、書かれた順の組にする。
func limitEntries(pairs ...string) []permission.LimitEntry {
	var entries []permission.LimitEntry
	for i := 0; i+1 < len(pairs); i += 2 {
		entries = append(entries, permission.LimitEntry{Name: pairs[i], Level: pairs[i+1]})
	}
	return entries
}
