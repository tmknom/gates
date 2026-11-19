package main

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// The Relation values of the Traceability Policy.
const (
	satisfies       = "Satisfies"
	narrows         = "Narrows"
	accepts         = "Accepts"
	possiblyRejects = "Possibly Rejects"
	none            = "None"
	// noClause is the Clause and the Note written when there is nothing to write.
	noClause = "なし"
)

var relationKinds = []string{satisfies, narrows, accepts, possiblyRejects, none}

var (
	relationHeader   = []string{"Relation", "Clause", "Note"}
	notAppliedHeader = []string{"Clause", "Reason"}
)

// notAppliedSection is the heading of the table of the standard clauses that
// GATES does not apply.
const notAppliedSection = "Standard Clauses Not Applied"

var (
	definitionHeading = regexp.MustCompile(`^#{1,6} (` + idSyntax + `)(?: |$)`)
	headingLine       = regexp.MustCompile(`^#{1,6}(?: (.*))?$`)
	// fieldItem matches an item such as "- **Level**：MUST".
	fieldItem = regexp.MustCompile(`^- \*\*([^*]+)\*\*：(.*)$`)
)

// definition is a heading that starts with an ID.
type definition struct {
	occurrence
	// fields are the "- **Name**：value" items just below the heading, such
	// as the Level, Actor, and Evidence of a clause.
	fields map[string]string
}

// relation is one row of the relation table of a definition.
type relation struct {
	definition string
	kind       string
	clause     string
	note       string
	path       string
	line       int
}

type markdownDocument struct {
	definitions []definition
	references  []occurrence
	relations   []relation
	// notApplied are the rows of the tables of standard clauses not applied.
	notApplied []occurrence
	findings   []finding
}

// markdownLine is a line of a document. Fenced code blocks hold examples of
// the formats, so nothing in them counts.
type markdownLine struct {
	number int
	text   string
	code   bool
}

// fence is the opening of a fenced code block. Only a run of the same
// character at least as long, with nothing after it, closes the block.
type fence struct {
	char   byte
	length int
}

// markdownLines splits a document into lines and marks the lines of fenced
// code blocks, fences included. Indentation is ignored, so that a fence in a
// list item counts as well.
func markdownLines(content string) []markdownLine {
	var result []markdownLine
	var open *fence
	for i, text := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(text)
		f, info, isFence := fenceOf(trimmed)
		switch {
		case open == nil && isFence:
			open = &f
		case open != nil && isFence && f.char == open.char && f.length >= open.length && info == "":
			result = append(result, markdownLine{number: i + 1, text: text, code: true})
			open = nil
			continue
		}
		result = append(result, markdownLine{number: i + 1, text: text, code: open != nil})
	}
	return result
}

// fenceOf reports whether a trimmed line is a fence of three or more
// backticks or tildes, and returns the text after the run.
func fenceOf(trimmed string) (fence, string, bool) {
	if trimmed == "" || (trimmed[0] != '`' && trimmed[0] != '~') {
		return fence{}, "", false
	}
	char := trimmed[0]
	n := len(trimmed) - len(strings.TrimLeft(trimmed, string(char)))
	info := strings.TrimSpace(trimmed[n:])
	if n < 3 || (char == '`' && strings.Contains(info, "`")) {
		return fence{}, "", false
	}
	return fence{char: char, length: n}, info, true
}

// parseMarkdown reads the definitions, references, and tables of a document.
func parseMarkdown(path, content string) markdownDocument {
	var doc markdownDocument
	report := func(line int, format string, args ...any) {
		doc.findings = append(doc.findings, finding{path: path, line: line, message: fmt.Sprintf(format, args...)})
	}
	all := markdownLines(content)
	var current *definition
	section := ""
	fieldsOpen := false
	table := ""
	for i, l := range all {
		if l.code {
			table = ""
			fieldsOpen = false
			continue
		}
		trimmed := strings.TrimSpace(l.text)
		if m := headingLine.FindStringSubmatch(trimmed); m != nil {
			table = ""
			section = strings.TrimSpace(m[1])
			current = nil
			fieldsOpen = false
			if d := definitionHeading.FindStringSubmatch(l.text); d != nil {
				doc.definitions = append(doc.definitions, definition{id: d[1], path: path, line: l.number})
				current = &doc.definitions[len(doc.definitions)-1]
				fieldsOpen = true
				continue
			}
		}
		for _, id := range findIDs(l.text) {
			doc.references = append(doc.references, occurrence{id: id, path: path, line: l.number})
		}
		if fieldsOpen && current != nil {
			if m := fieldItem.FindStringSubmatch(trimmed); m != nil {
				if current.fields == nil {
					current.fields = map[string]string{}
				}
				current.fields[m[1]] = strings.TrimSpace(m[2])
			} else if trimmed != "" || current.fields != nil {
				fieldsOpen = false
			}
		}
		cells, isRow := tableCells(trimmed)
		if !isRow {
			table = ""
			continue
		}
		isHeader := i+1 < len(all) && !all[i+1].code && isSeparatorLine(all[i+1].text)
		switch {
		case isHeader && slices.Equal(cells, relationHeader):
			if current == nil {
				report(l.number, "relation table is not under a definition heading")
				continue
			}
			table = "relation"
		case isHeader && slices.Equal(cells, notAppliedHeader) && section == notAppliedSection:
			table = "notApplied"
		case isHeader && current != nil && isGOJ(current.id) && looksLikeRelationHeader(cells):
			report(l.number, "table header %q is not %q", strings.Join(cells, " | "), strings.Join(relationHeader, " | "))
		case isSeparator(cells):
		case table == "relation":
			if r, ok := parseRelationRow(cells); ok {
				r.definition, r.path, r.line = current.id, path, l.number
				doc.relations = append(doc.relations, r)
			} else if !slices.Contains(relationKinds, cells[0]) {
				report(l.number, "unknown Relation %q", cells[0])
			} else {
				report(l.number, "relation row has %d cells, want Relation, Clause, and Note", len(cells))
			}
		case table == "notApplied":
			if !isID(cells[0]) {
				report(l.number, "Clause %q is not an ID", cells[0])
				continue
			}
			doc.notApplied = append(doc.notApplied, occurrence{id: cells[0], path: path, line: l.number})
		}
	}
	return doc
}

// parseRelationRow reads a row of a relation table. It does not accept a row
// with an unknown Relation or a missing cell.
func parseRelationRow(cells []string) (relation, bool) {
	if len(cells) != len(relationHeader) || !slices.Contains(relationKinds, cells[0]) {
		return relation{}, false
	}
	return relation{kind: cells[0], clause: cells[1], note: cells[2]}, true
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

// looksLikeRelationHeader reports a header that names a Relation column, so
// that a misspelled relation table under a GOJ is not silently left out of
// the check. Other definitions have tables of their own, such as the
// Relationship table of an AC.
func looksLikeRelationHeader(cells []string) bool {
	for _, c := range cells {
		if strings.HasPrefix(strings.ToLower(c), "relation") {
			return true
		}
	}
	return false
}

func isSeparatorLine(text string) bool {
	cells, isRow := tableCells(strings.TrimSpace(text))
	return isRow && isSeparator(cells)
}

func isSeparator(cells []string) bool {
	for _, c := range cells {
		if strings.Trim(c, "-: ") != "" {
			return false
		}
	}
	return true
}
