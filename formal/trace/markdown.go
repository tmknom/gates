package main

import (
	"fmt"
	"regexp"
	"strings"
)

// relationKinds are the Relation values of the Traceability Policy.
var relationKinds = map[string]bool{
	"Satisfies":        true,
	"Narrows":          true,
	"Accepts":          true,
	"Possibly Rejects": true,
	"None":             true,
}

var definitionHeading = regexp.MustCompile(`^#{1,6} (` + idSyntax + `)(?: |$)`)

// relation is one row of the relation table of a definition.
type relation struct {
	definition string
	kind       string
	clause     string
	path       string
	line       int
}

type markdownDocument struct {
	definitions []occurrence
	references  []occurrence
	relations   []relation
	findings    []finding
}

// parseMarkdown reads the definitions, references, and relation tables of a
// document. Fenced code blocks hold examples of the formats, so nothing in
// them counts.
func parseMarkdown(path, content string) markdownDocument {
	var doc markdownDocument
	inFence := false
	inTable := false
	current := ""
	for i, text := range strings.Split(content, "\n") {
		line := i + 1
		trimmed := strings.TrimSpace(text)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			inTable = false
			current = ""
			if m := definitionHeading.FindStringSubmatch(text); m != nil {
				current = m[1]
				doc.definitions = append(doc.definitions, occurrence{id: current, path: path, line: line})
				continue
			}
		}
		for _, id := range findIDs(text) {
			doc.references = append(doc.references, occurrence{id: id, path: path, line: line})
		}
		cells, isRow := tableCells(trimmed)
		switch {
		case !isRow:
			inTable = false
		case isRelationHeader(cells):
			if current == "" {
				doc.findings = append(doc.findings, finding{path: path, line: line, message: "relation table is not under a definition heading"})
				continue
			}
			inTable = true
		case inTable && !isSeparator(cells):
			kind := cells[0]
			if !relationKinds[kind] {
				doc.findings = append(doc.findings, finding{path: path, line: line, message: fmt.Sprintf("unknown Relation %q", kind)})
				continue
			}
			clause := ""
			if len(cells) > 1 {
				clause = cells[1]
			}
			doc.relations = append(doc.relations, relation{definition: current, kind: kind, clause: clause, path: path, line: line})
		}
	}
	return doc
}

// tableCells splits a Markdown table row into its trimmed cells.
func tableCells(trimmed string) ([]string, bool) {
	if !strings.HasPrefix(trimmed, "|") || !strings.HasSuffix(trimmed, "|") || len(trimmed) < 2 {
		return nil, false
	}
	cells := strings.Split(trimmed[1:len(trimmed)-1], "|")
	for i, c := range cells {
		cells[i] = strings.TrimSpace(c)
	}
	return cells, true
}

func isRelationHeader(cells []string) bool {
	return len(cells) == 3 && cells[0] == "Relation" && cells[1] == "Clause" && cells[2] == "Note"
}

func isSeparator(cells []string) bool {
	for _, c := range cells {
		if strings.Trim(c, "-: ") != "" {
			return false
		}
	}
	return true
}
