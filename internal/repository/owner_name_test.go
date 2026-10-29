package repository_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/repository"
)

func TestParseOwnerName(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "mixed case letters and digits are accepted", input: "Example0rg9"},
		{name: "single character is accepted", input: "a"},
		{name: "leading hyphen is accepted", input: "-lead"},
		{name: "trailing hyphen is accepted", input: "trail-"},
		{name: "consecutive hyphens are accepted", input: "dou--ble"},
		{name: "name longer than 39 characters is accepted", input: strings.Repeat("a", 40)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repository.ParseOwnerName(tt.input)
			if err != nil {
				t.Fatalf("ParseOwnerName(%q) returned error: %v", tt.input, err)
			}
			if got.String() != tt.input {
				t.Errorf("String() = %q, want %q", got.String(), tt.input)
			}
		})
	}
}

func TestParseOwnerName_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "empty name returns ErrInvalidOwnerName", input: ""},
		{name: "underscore returns ErrInvalidOwnerName", input: "ex_ample"},
		{name: "slash returns ErrInvalidOwnerName", input: "example/org"},
		{name: "Kelvin sign returns ErrInvalidOwnerName", input: "\u212Aowner"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := repository.ParseOwnerName(tt.input)
			if !errors.Is(err, repository.ErrInvalidOwnerName) {
				t.Errorf("ParseOwnerName(%q) error = %v, want %v", tt.input, err, repository.ErrInvalidOwnerName)
			}
		})
	}
}

func TestOwnerName_Equal(t *testing.T) {
	tests := []struct {
		name  string
		a     string
		b     string
		equal bool
	}{
		{name: "same spelling is equal", a: "example-org", b: "example-org", equal: true},
		{name: "names differing only in ASCII case are equal", a: "Example-ORG", b: "example-org", equal: true},
		{name: "names differing in one character are not equal", a: "example-org", b: "example-orh", equal: false},
		{name: "names differing in length are not equal", a: "example-org", b: "example-org1", equal: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := mustParseOwnerName(t, tt.a)
			b := mustParseOwnerName(t, tt.b)
			if got := a.Equal(b); got != tt.equal {
				t.Errorf("%q.Equal(%q) = %v, want %v", tt.a, tt.b, got, tt.equal)
			}
		})
	}
}

func mustParseOwnerName(t *testing.T, s string) repository.OwnerName {
	t.Helper()
	o, err := repository.ParseOwnerName(s)
	if err != nil {
		t.Fatalf("ParseOwnerName(%q) returned error: %v", s, err)
	}
	return o
}
