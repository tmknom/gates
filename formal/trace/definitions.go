package main

import (
	"fmt"
	"slices"
)

// misplacedDefinitions reports an ID defined more than once, and an ID
// defined outside the documents that the table of ID kinds gives its prefix.
// A reference reads the definition by its ID alone, so a second definition
// or one in an unexpected document leaves the reference ambiguous.
func (l *links) misplacedDefinitions() []finding {
	kinds := make(map[string]idKind, len(l.idKinds))
	for _, k := range l.idKinds {
		kinds[k.prefix] = k
	}
	first := make(map[string]definition, len(l.definitions))
	var findings []finding
	for _, d := range l.definitions {
		if prior, ok := first[d.id]; ok {
			findings = append(findings, finding{path: d.path, line: d.line, message: fmt.Sprintf("%s is also defined at %s", d.id, prior.location())})
			continue
		}
		first[d.id] = d
		prefix := prefixOf(d.id)
		if k, ok := kinds[prefix]; ok && !k.defines(d.path) {
			findings = append(findings, finding{path: d.path, line: d.line, message: fmt.Sprintf("%s is defined outside the documents for %s- IDs", d.id, prefix)})
		}
	}
	return findings
}

// idKindsMismatch reports a difference between the prefixes in the table of
// ID kinds and the prefixes the check reads.
func (l *links) idKindsMismatch() []finding {
	if len(l.idKinds) == 0 {
		return []finding{{path: idKindsDocument, line: 1, message: "no table of ID kinds"}}
	}
	var findings []finding
	inTable := make([]string, 0, len(l.idKinds))
	for _, k := range l.idKinds {
		inTable = append(inTable, k.prefix)
		if !slices.Contains(idPrefixes, k.prefix) {
			findings = append(findings, finding{path: idKindsDocument, line: k.line, message: fmt.Sprintf("%s- in the table of ID kinds is not an ID kind the trace check reads", k.prefix)})
		}
	}
	for _, p := range idPrefixes {
		if !slices.Contains(inTable, p) {
			findings = append(findings, finding{path: idKindsDocument, line: 1, message: fmt.Sprintf("the table of ID kinds has no row for %s-", p)})
		}
	}
	return findings
}
