package main

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
)

type annotationKind string

const (
	formalizes   annotationKind = "formalizes"
	checks       annotationKind = "checks"
	unformalized annotationKind = "unformalized"
)

// annotation is a trace: line comment that links a test, a definition of a
// formal model, or a check to the IDs it lists.
type annotation struct {
	kind annotationKind
	ids  []string
	path string
	line int
	// universal is set on a trace:checks of a formal check that passes only
	// when the solver finds no counterexample: an SMT-LIB check expecting
	// unsat for every check-sat, or an Alloy check command with expect 0.
	// Only such a check shows that a relation holds for every input.
	universal bool
	// witness is set on a trace:checks of a formal check that passes only
	// when the solver finds an instance: an SMT-LIB check expecting sat for
	// every check-sat, or an Alloy command with expect 1. A Possibly Rejects
	// relation needs such a check to show an issuance that the GOJ rejects.
	// Alloy exits with 0 whatever it finds unless the command has expect, so
	// a command without expect is neither universal nor a witness.
	witness bool
}

// inModel reports whether the annotation is in a formal model rather than in
// a Go test or a Terraform test.
func (a annotation) inModel() bool {
	ext := path.Ext(a.path)
	return ext == ".smt2" || ext == ".als"
}

func (a annotation) inGoTest() bool {
	return strings.HasSuffix(a.path, "_test.go")
}

func (a annotation) lists(ids ...string) bool {
	for _, id := range ids {
		if !slices.Contains(a.ids, id) {
			return false
		}
	}
	return true
}

// commentSyntax is how annotations and the lines they must precede are
// written in one kind of file.
type commentSyntax struct {
	annotation *regexp.Regexp
	// definition matches the line a trace:formalizes must precede.
	definition *regexp.Regexp
	// command matches the line a trace:checks of a model must precede,
	// capturing the command and the number after expect. When nil, the
	// checks annotation goes on the line after the "; expect:" line instead.
	command *regexp.Regexp
	// test matches the line a trace:checks of a test must precede.
	test *regexp.Regexp
}

var (
	smtSyntax = commentSyntax{
		annotation: regexp.MustCompile(`^\s*;+\s*trace:(\S*)(.*)$`),
		definition: regexp.MustCompile(`^\(define-fun\s`),
	}
	alloySyntax = commentSyntax{
		annotation: regexp.MustCompile(`^\s*//\s*trace:(\S*)(.*)$`),
		definition: regexp.MustCompile(`^\s*(?:private\s+)?(?:pred|fact|fun)\b`),
		command:    regexp.MustCompile(`^\s*(?:\w+\s*:\s*)?(check|run)\b(?:.*\bexpect\s+(\d+))?`),
	}
	// Go reads //trace: without a space as a directive, which keeps it out of
	// the doc comment, so only that spelling is an annotation.
	goSyntax = commentSyntax{
		annotation: regexp.MustCompile(`^\s*//trace:(\S*)(.*)$`),
		test:       regexp.MustCompile(`^func Test`),
	}
	terraformSyntax = commentSyntax{
		annotation: regexp.MustCompile(`^\s*#\s*trace:(\S*)(.*)$`),
		test:       regexp.MustCompile(`^\s*run\s+"`),
	}
	expectLine = regexp.MustCompile(`^; expect:`)
	// allUnsat matches an expect line whose verdicts are all unsat.
	allUnsat = regexp.MustCompile(`^; expect:(?:\s+unsat)+\s*$`)
	// allSat matches an expect line whose verdicts are all sat.
	allSat = regexp.MustCompile(`^; expect:(?:\s+sat)+\s*$`)
)

// syntaxFor returns the comment syntax of a file that may hold annotations.
func syntaxFor(name string) (commentSyntax, bool) {
	switch path.Ext(name) {
	case ".smt2":
		return smtSyntax, true
	case ".als":
		return alloySyntax, true
	case ".go":
		return goSyntax, true
	case ".hcl":
		return terraformSyntax, true
	}
	return commentSyntax{}, false
}

// parseAnnotations reads the annotations of a file whose name syntaxFor accepts.
func parseAnnotations(name, content string) ([]annotation, []finding) {
	syntax, _ := syntaxFor(name)
	textLines := strings.Split(content, "\n")
	var annotations []annotation
	var findings []finding
	report := func(line int, format string, args ...any) {
		findings = append(findings, finding{path: name, line: line, message: fmt.Sprintf(format, args...)})
	}
	for i, text := range textLines {
		m := syntax.annotation.FindStringSubmatch(text)
		if m == nil {
			continue
		}
		line := i + 1
		kind := annotationKind(m[1])
		tokens := strings.Fields(m[2])
		next := ""
		if i+1 < len(textLines) {
			next = textLines[i+1]
		}
		switch kind {
		case formalizes, checks:
			if len(tokens) == 0 {
				report(line, "trace:%s lists no ID", kind)
				continue
			}
			a := annotation{kind: kind, path: name, line: line}
			for _, token := range tokens {
				if !isID(token) {
					report(line, "%q is not an ID", token)
					continue
				}
				a.ids = append(a.ids, token)
			}
			if kind == formalizes && syntax.definition != nil && !syntax.definition.MatchString(next) {
				report(line, "trace:formalizes must be on the line just before a definition")
			}
			if kind == checks && syntax.test != nil && !syntax.test.MatchString(next) {
				report(line, "trace:checks must be on the line just before a test")
			}
			if kind == checks && syntax.definition != nil {
				if syntax.command != nil {
					c := syntax.command.FindStringSubmatch(next)
					if c == nil {
						report(line, "trace:checks must be on the line just before a check or run command")
					} else {
						command, expect := c[1], c[2]
						a.universal = command == "check" && expect == "0"
						a.witness = expect == "1"
					}
				} else {
					if line != 2 || !expectLine.MatchString(textLines[0]) {
						report(line, `trace:checks must be on the line after "; expect:"`)
					}
					a.universal = line == 2 && allUnsat.MatchString(textLines[0])
					a.witness = line == 2 && allSat.MatchString(textLines[0])
				}
			}
			annotations = append(annotations, a)
		case unformalized:
			if len(tokens) == 0 || !isID(tokens[0]) {
				report(line, "trace:unformalized needs one ID and a reason")
				continue
			}
			annotations = append(annotations, annotation{kind: kind, ids: tokens[:1], path: name, line: line})
			if len(tokens) < 2 {
				report(line, "trace:unformalized needs one ID and a reason")
			}
		default:
			report(line, "unknown annotation %q", "trace:"+string(kind))
		}
	}
	return annotations, findings
}
