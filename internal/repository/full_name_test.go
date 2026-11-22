package repository_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/repository"
)

func TestParseFullName(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		ownerName      string
		repositoryName string
	}{
		{name: "requested spelling is kept", input: "Example-Org/Repo.A_1", ownerName: "Example-Org", repositoryName: "Repo.A_1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repository.ParseFullName(tt.input)
			if err != nil {
				t.Fatalf("ParseFullName(%q) returned error: %v", tt.input, err)
			}
			if got.OwnerName().String() != tt.ownerName {
				t.Errorf("OwnerName() = %q, want %q", got.OwnerName(), tt.ownerName)
			}
			if got.RepositoryName() != tt.repositoryName {
				t.Errorf("RepositoryName() = %q, want %q", got.RepositoryName(), tt.repositoryName)
			}
			if got.String() != tt.input {
				t.Errorf("String() = %q, want %q", got.String(), tt.input)
			}
		})
	}
}

func TestParseFullName_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "value without slash returns ErrInvalidFullName", input: "repo"},
		{name: "value with two slashes returns ErrInvalidFullName", input: "org/repo/extra"},
		{name: "empty owner name returns ErrInvalidFullName", input: "/repo"},
		{name: "empty repository name returns ErrInvalidFullName", input: "org/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := repository.ParseFullName(tt.input)
			if !errors.Is(err, repository.ErrInvalidFullName) {
				t.Errorf("ParseFullName(%q) error = %v, want %v", tt.input, err, repository.ErrInvalidFullName)
			}
		})
	}
}

// リポジトリ名の規則は ValidateRepositoryName が検証する。ここでは、その条件を完全名の誤りとして包むことを確かめる
func TestParseFullName_InvalidRepositoryName(t *testing.T) {
	_, err := repository.ParseFullName("org/repo!")
	const want = "invalid full name: repository name must consist of ASCII letters, digits, '-', '_', or '.'"
	if !errors.Is(err, repository.ErrInvalidFullName) || err.Error() != want {
		t.Errorf("error = %v, want %q wrapping %v", err, want, repository.ErrInvalidFullName)
	}
}

func TestParseFullName_ErrorDoesNotContainInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "invalid owner name is not in error message", input: "secret_owner/repo"},
		{name: "invalid repository name is not in error message", input: "org/secret!repo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := repository.ParseFullName(tt.input)
			if err == nil {
				t.Fatalf("ParseFullName(%q) returned no error", tt.input)
			}
			if strings.Contains(err.Error(), "secret") {
				t.Errorf("error message %q contains the input", err.Error())
			}
		})
	}
}

func TestFullName_Equal(t *testing.T) {
	tests := []struct {
		name  string
		a     string
		b     string
		equal bool
	}{
		{name: "same spelling is equal", a: "example-org/repo-a", b: "example-org/repo-a", equal: true},
		{name: "owner names differing only in ASCII case are equal", a: "EXAMPLE-ORG/repo-a", b: "example-org/repo-a", equal: true},
		{name: "repository names differing only in ASCII case are equal", a: "example-org/Repo-A", b: "example-org/repo-a", equal: true},
		{name: "repository names differing in one character are not equal", a: "example-org/repo-a", b: "example-org/repo-b", equal: false},
		{name: "owner names differing in one character are not equal", a: "example-org/repo-a", b: "example-orh/repo-a", equal: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := mustParseFullName(t, tt.a)
			b := mustParseFullName(t, tt.b)
			if got := a.Equal(b); got != tt.equal {
				t.Errorf("%q.Equal(%q) = %v, want %v", tt.a, tt.b, got, tt.equal)
			}
		})
	}
}

func mustParseFullName(t *testing.T, s string) repository.FullName {
	t.Helper()
	f, err := repository.ParseFullName(s)
	if err != nil {
		t.Fatalf("ParseFullName(%q) returned error: %v", s, err)
	}
	return f
}
