package main

import "regexp"

// The prefixes are the ID kinds of the Traceability Policy. An ID ends at a
// character that cannot continue it, so a wildcard such as RFC7519-EXP-* or a
// prefix written alone is not taken for an ID.
const idSyntax = `(?:FR|QR|AC|GOJ|AOJ|GAJ|RULE|INV|RFC7515|RFC7517|RFC7518|RFC7797|RFC9964|RFC7159|RFC7519|RFC8725|OIDCC|OIDCD|GH-OIDC|GH-ACCOUNT|GH-APP|GH-ACTIONS)-[A-Z0-9]+(?:-[A-Z0-9]+)*`

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
