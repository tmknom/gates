package main

import (
	"slices"
	"testing"
)

func TestFindIDs(t *testing.T) {
	cases := []struct {
		name string
		line string
		want []string
	}{
		{name: "ID followed by Japanese text is found", line: "GOJ-AUD は必須", want: []string{"GOJ-AUD"}},
		{name: "every ID on a line is found in order", line: "GOJ-EXP、GOJ-IAT と RFC7519-EXP-REJECT", want: []string{"GOJ-EXP", "GOJ-IAT", "RFC7519-EXP-REJECT"}},
		{name: "ID in inline code is found", line: "`GH-OIDC-AUD-DEFAULT`", want: []string{"GH-OIDC-AUD-DEFAULT"}},
		{name: "prefix alone is not an ID", line: "`GOJ-` で始まる", want: nil},
		{name: "wildcard is not an ID", line: "RFC7519-EXP-* と GOJ-HEADER-*", want: nil},
		{name: "ID inside a longer word is not an ID", line: "XGOJ-AUD", want: nil},
		{name: "ID followed by a lowercase letter is not an ID", line: "GOJ-AUDx", want: nil},
		{name: "unknown prefix is not an ID", line: "RFC-EXP-TIME と GH-ISS", want: nil},
		{name: "lowercase anchor is not an ID", line: "(#goj-aud-aud)", want: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := findIDs(tc.line)
			if !slices.Equal(got, tc.want) {
				t.Errorf("findIDs(%q) = %q, want %q", tc.line, got, tc.want)
			}
		})
	}
}

func TestIsID(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{name: "whole ID is an ID", text: "RFC8725-AUD-VALIDATE", want: true},
		{name: "ID with trailing text is not an ID", text: "GOJ-AUD,", want: false},
		{name: "underscore spelling is not an ID", text: "GOJ_AUD", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isID(tc.text); got != tc.want {
				t.Errorf("isID(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}
