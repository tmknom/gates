package repository_test

import (
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/repository"
)

func TestValidateRepositoryName(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "letters, digits, hyphen, underscore, and period are accepted", input: "Repo-A_1.x"},
		{name: "name of 100 characters is accepted", input: strings.Repeat("r", 100)},
		{name: "single period is accepted", input: "."},
		{name: "double period is accepted", input: ".."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := repository.ValidateRepositoryName(tt.input); err != nil {
				t.Errorf("ValidateRepositoryName(%q) returned error: %v", tt.input, err)
			}
		})
	}
}

func TestValidateRepositoryName_Invalid(t *testing.T) {
	const (
		wantCharacters = "must consist of ASCII letters, digits, '-', '_', or '.'"
		wantLength     = "must be 1 to 100 characters"
	)
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty name returns the length condition", input: "", want: wantLength},
		{name: "name of 101 characters returns the length condition", input: strings.Repeat("r", 101), want: wantLength},
		{name: "name with slash returns the character condition", input: "owner/repo", want: wantCharacters},
		{name: "name with Kelvin sign returns the character condition", input: "\u212Aepo", want: wantCharacters},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repository.ValidateRepositoryName(tt.input)
			if err == nil || err.Error() != tt.want {
				t.Errorf("ValidateRepositoryName(%q) error = %v, want %q", tt.input, err, tt.want)
			}
		})
	}
}
