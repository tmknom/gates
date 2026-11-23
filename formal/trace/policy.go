package main

import (
	"path"
	"regexp"
	"slices"
	"strings"
)

// The Traceability Policy and the OIDC JWT Claims hold the lists the
// check reads besides the definitions: the ID kinds and the Actors that are
// GATES roles.
const (
	idKindsDocument = "docs/contributing/traceability.md"
	actorsDocument  = "docs/design/oidc-jwt-claims.md"
	actorSection    = "GATES の役割に当たる Actor"
)

// Column names of the table of ID kinds.
const (
	prefixColumn   = "接頭辞"
	documentColumn = "定めるドキュメント"
)

var (
	boldText   = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	inlineCode = regexp.MustCompile("`([^`]+)`")
)

// idKind is a row of the table of ID kinds: an ID prefix and the documents
// that may define IDs with it.
type idKind struct {
	prefix    string
	documents []string
	line      int
}

// bareNameDirectories hold the documents that the table of ID kinds names by
// their file name alone.
var bareNameDirectories = []string{"docs/spec", "docs/design"}

// defines reports whether the document at name may define IDs of the kind.
func (k idKind) defines(name string) bool {
	for _, d := range k.documents {
		if name == d || (!strings.Contains(d, "/") && path.Base(name) == d && slices.Contains(bareNameDirectories, path.Dir(name))) {
			return true
		}
	}
	return false
}

// parseActors returns the Actors listed as "**Actor**：説明" items in the
// section of the Actors that are GATES roles.
func parseActors(content string) []string {
	var actors []string
	inSection := false
	for _, l := range markdownLines(content) {
		if l.code {
			continue
		}
		trimmed := strings.TrimSpace(l.text)
		if m := headingLine.FindStringSubmatch(trimmed); m != nil {
			inSection = strings.TrimSpace(m[1]) == actorSection
			continue
		}
		if !inSection || !strings.HasPrefix(trimmed, "- ") {
			continue
		}
		names, _, _ := strings.Cut(trimmed, "：")
		for _, m := range boldText.FindAllStringSubmatch(names, -1) {
			actors = append(actors, m[1])
		}
	}
	return actors
}

// parseIDKinds reads the table of ID kinds.
func parseIDKinds(content string) []idKind {
	var kinds []idKind
	inTable := false
	prefixAt, documentAt := -1, -1
	for _, l := range markdownLines(content) {
		cells, isRow := tableCells(strings.TrimSpace(l.text))
		if l.code || !isRow {
			inTable = false
			continue
		}
		if !inTable {
			inTable = true
			prefixAt, documentAt = slices.Index(cells, prefixColumn), slices.Index(cells, documentColumn)
			continue
		}
		if prefixAt < 0 || documentAt < 0 || isSeparator(cells) || len(cells) <= max(prefixAt, documentAt) {
			continue
		}
		kind := idKind{prefix: strings.TrimSuffix(strings.Trim(cells[prefixAt], "`"), "-"), line: l.number}
		for _, m := range inlineCode.FindAllStringSubmatch(cells[documentAt], -1) {
			kind.documents = append(kind.documents, m[1])
		}
		kinds = append(kinds, kind)
	}
	return kinds
}
