package main

import (
	"cmp"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
)

// Roots of the files the check reads. Documents define the IDs; models and
// tests refer to them by annotations.
const (
	docsRoot   = "docs"
	formalRoot = "formal"
)

// annotationRoots hold tests and models outside formal/ that declare
// trace:checks.
var annotationRoots = []string{"internal", ".github", "terraform/tests"}

// occurrence is where an ID is defined or referred to.
type occurrence struct {
	id   string
	path string
	line int
}

// finding is a missing or broken link between a definition and its references.
type finding struct {
	path    string
	line    int
	message string
}

func (f finding) String() string {
	return fmt.Sprintf("%s:%d: %s", f.path, f.line, f.message)
}

// links is everything the check reads from the repository.
type links struct {
	definitions []occurrence
	references  []occurrence
	relations   []relation
	annotations []annotation
	findings    []finding
}

// check reports every ID that is referred to but not defined, every malformed
// annotation, and every Satisfies relation that no formal check declares.
func check(fsys fs.FS) ([]finding, error) {
	var l links
	if err := walk(fsys, docsRoot, func(name, content string) { l.addDocument(name, content) }); err != nil {
		return nil, err
	}
	if err := walk(fsys, formalRoot, func(name, content string) { l.addFormal(name, content) }); err != nil {
		return nil, err
	}
	for _, root := range annotationRoots {
		if err := walk(fsys, root, func(name, content string) { l.addAnnotations(name, content) }); err != nil {
			return nil, err
		}
	}
	findings := slices.Concat(l.findings, l.undefinedReferences(), l.uncheckedSatisfies())
	slices.SortFunc(findings, func(a, b finding) int {
		return cmp.Or(cmp.Compare(a.path, b.path), cmp.Compare(a.line, b.line), cmp.Compare(a.message, b.message))
	})
	return slices.Compact(findings), nil
}

// walk calls read with the content of every regular file under root.
func walk(fsys fs.FS, root string, read func(name, content string)) error {
	return fs.WalkDir(fsys, root, func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		content, err := fs.ReadFile(fsys, name)
		if err != nil {
			return err
		}
		read(name, string(content))
		return nil
	})
}

func (l *links) addDocument(name, content string) {
	if path.Ext(name) != ".md" {
		return
	}
	doc := parseMarkdown(name, content)
	l.definitions = append(l.definitions, doc.definitions...)
	l.references = append(l.references, doc.references...)
	l.relations = append(l.relations, doc.relations...)
	l.findings = append(l.findings, doc.findings...)
}

// addFormal reads a model or a document under formal/. IDs anywhere in a
// model, comments included, are references, so a model cannot cite an ID
// that no document defines. Definitions stay in docs/.
func (l *links) addFormal(name, content string) {
	switch path.Ext(name) {
	case ".md":
		doc := parseMarkdown(name, content)
		l.references = append(l.references, doc.references...)
		l.findings = append(l.findings, doc.findings...)
	case ".smt2", ".als":
		for i, text := range strings.Split(content, "\n") {
			for _, id := range findIDs(text) {
				l.references = append(l.references, occurrence{id: id, path: name, line: i + 1})
			}
		}
		l.addAnnotations(name, content)
	case ".go":
		l.addAnnotations(name, content)
	}
}

func (l *links) addAnnotations(name, content string) {
	if _, ok := syntaxFor(name); !ok {
		return
	}
	annotations, findings := parseAnnotations(name, content)
	l.annotations = append(l.annotations, annotations...)
	l.findings = append(l.findings, findings...)
	for _, a := range annotations {
		for _, id := range a.ids {
			l.references = append(l.references, occurrence{id: id, path: a.path, line: a.line})
		}
	}
}

func (l *links) undefinedReferences() []finding {
	defined := make(map[string]bool, len(l.definitions))
	for _, d := range l.definitions {
		defined[d.id] = true
	}
	var findings []finding
	for _, r := range l.references {
		if !defined[r.id] {
			findings = append(findings, finding{path: r.path, line: r.line, message: "undefined ID " + r.id})
		}
	}
	return findings
}

// uncheckedSatisfies reports Satisfies relations that no universal formal
// check declares. A Go test or a check expecting a witness does not count,
// because only a check that finds no counterexample shows that every Claims
// Set the GOJ accepts satisfies the clause.
func (l *links) uncheckedSatisfies() []finding {
	var findings []finding
	for _, r := range l.relations {
		if r.kind != "Satisfies" || l.universallyChecked(r.definition, r.clause) {
			continue
		}
		findings = append(findings, finding{
			path:    r.path,
			line:    r.line,
			message: fmt.Sprintf("Satisfies %s %s has no trace:checks listing both IDs on a check expecting no counterexample", r.definition, r.clause),
		})
	}
	return findings
}

func (l *links) universallyChecked(definition, clause string) bool {
	for _, a := range l.annotations {
		if a.universal && slices.Contains(a.ids, definition) && slices.Contains(a.ids, clause) {
			return true
		}
	}
	return false
}
