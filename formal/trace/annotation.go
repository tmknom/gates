package main

import (
	"fmt"
	"path"
	"regexp"
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
	// universal is set on a trace:checks of a formal check that holds only
	// when the solver finds no counterexample: an SMT-LIB check expecting
	// unsat for every check-sat, or an Alloy check command. Only such a check
	// shows that a relation holds for every input.
	universal bool
}

// commentSyntax is how annotations and the lines they must precede are
// written in one kind of file.
type commentSyntax struct {
	annotation *regexp.Regexp
	// definition matches the line a trace:formalizes must precede.
	definition *regexp.Regexp
	// command matches the line a trace:checks must precede. When nil, the
	// checks annotation goes on the line after the "; expect:" line instead.
	command *regexp.Regexp
	// universalCommand matches a command that finds no counterexample when it passes.
	universalCommand *regexp.Regexp
}

var (
	smtSyntax = commentSyntax{
		annotation: regexp.MustCompile(`^\s*;+\s*trace:(\S*)(.*)$`),
		definition: regexp.MustCompile(`^\(define-fun\s`),
	}
	alloySyntax = commentSyntax{
		annotation:       regexp.MustCompile(`^\s*//\s*trace:(\S*)(.*)$`),
		definition:       regexp.MustCompile(`^\s*(?:private\s+)?(?:pred|fact|fun)\b`),
		command:          regexp.MustCompile(`^\s*(?:\w+\s*:\s*)?(?:check|run)\b`),
		universalCommand: regexp.MustCompile(`^\s*(?:\w+\s*:\s*)?check\b`),
	}
	// Go reads //trace: without a space as a directive, which keeps it out of
	// the doc comment, so only that spelling is an annotation.
	goSyntax = commentSyntax{
		annotation: regexp.MustCompile(`^\s*//trace:(\S*)(.*)$`),
	}
	terraformSyntax = commentSyntax{
		annotation: regexp.MustCompile(`^\s*#\s*trace:(\S*)(.*)$`),
	}
	expectLine = regexp.MustCompile(`^; expect:`)
	// allUnsat matches an expect line whose verdicts are all unsat.
	allUnsat = regexp.MustCompile(`^; expect:(?:\s+unsat)+\s*$`)
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
			if kind == checks && syntax.definition != nil {
				if syntax.command != nil {
					if !syntax.command.MatchString(next) {
						report(line, "trace:checks must be on the line just before a check or run command")
					}
					a.universal = syntax.universalCommand.MatchString(next)
				} else {
					if line != 2 || !expectLine.MatchString(textLines[0]) {
						report(line, `trace:checks must be on the line after "; expect:"`)
					}
					a.universal = line == 2 && allUnsat.MatchString(textLines[0])
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
