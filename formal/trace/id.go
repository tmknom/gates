package main

import (
	"regexp"
	"slices"
	"strings"
)

// Prefixes of the ID kinds of the Traceability Policy. The table of ID kinds
// in that policy lists the same prefixes, and the check reports a difference.
var (
	standardClausePrefixes = []string{"RFC7159", "RFC7515", "RFC7517", "RFC7518", "RFC7519", "RFC7797", "RFC8725", "RFC9964", "OIDCC", "OIDCD"}
	githubClausePrefixes   = []string{"GH-OIDC", "GH-ACCOUNT", "GH-APP", "GH-ACTIONS"}
	idPrefixes             = slices.Concat([]string{"FR", "QR", "AC", "GOJ", "RULE", "INV"}, standardClausePrefixes, githubClausePrefixes)
)

const gojPrefix = "GOJ"

// An ID ends at a character that cannot continue it, so a wildcard such as
// RFC7519-EXP-* or a prefix written alone is not taken for an ID.
var idSyntax = `(?:` + strings.Join(idPrefixes, "|") + `)-[A-Z0-9]+(?:-[A-Z0-9]+)*`

var (
	// The last group captures a character that would continue the ID.
	idInLine = regexp.MustCompile(`(?:^|[^A-Za-z0-9_-])(` + idSyntax + `)([A-Za-z0-9_*-]?)`)
	wholeID  = regexp.MustCompile(`^` + idSyntax + `$`)
)

// findIDs returns the IDs written in line, in order.
func findIDs(line string) []string {
	var ids []string
	for _, m := range idInLine.FindAllStringSubmatch(line, -1) {
		id, continued := m[1], m[2]
		if continued != "" {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

func isID(token string) bool {
	return wholeID.MatchString(token)
}

// prefixOf returns the prefix of an ID. GH-OIDC and the other GitHub prefixes
// contain a hyphen, so the longest prefix that matches wins.
func prefixOf(id string) string {
	longest := ""
	for _, p := range idPrefixes {
		if strings.HasPrefix(id, p+"-") && len(p) > len(longest) {
			longest = p
		}
	}
	return longest
}

func isStandardClause(id string) bool {
	return slices.Contains(standardClausePrefixes, prefixOf(id))
}

func isGitHubClause(id string) bool {
	return slices.Contains(githubClausePrefixes, prefixOf(id))
}

func isClause(id string) bool {
	return isStandardClause(id) || isGitHubClause(id)
}

func isGOJ(id string) bool {
	return prefixOf(id) == gojPrefix
}
