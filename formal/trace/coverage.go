package main

import (
	"fmt"
	"slices"
)

// strongLevels are the Levels of a strong norm. A clause of another Level lets
// GATES choose not to follow it or imposes no duty.
var strongLevels = []string{"MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT"}

// uncoveredStrongNorms reports a strong norm of a GATES role that no Satisfies
// or Narrows row and no table of standard clauses not applied lists, a clause
// listed in both, and a clause in the table that is not such a norm.
func (l *links) uncoveredStrongNorms() []finding {
	if len(l.actors) == 0 {
		return []finding{{path: actorsDocument, line: 1, message: "no list of the Actors that are GATES roles"}}
	}
	strong := make(map[string]bool)
	for _, d := range l.definitions {
		if isStandardClause(d.id) && slices.Contains(strongLevels, d.fields["Level"]) && slices.Contains(l.actors, d.fields["Actor"]) {
			strong[d.id] = true
		}
	}
	related := make(map[string]bool)
	for _, r := range l.relations {
		if r.kind == satisfies || r.kind == narrows {
			related[r.clause] = true
		}
	}
	notApplied := make(map[string]bool)
	var findings []finding
	for _, n := range l.notApplied {
		notApplied[n.id] = true
		switch {
		case related[n.id]:
			findings = append(findings, finding{path: n.path, line: n.line, message: n.id + " is in the table of standard clauses not applied and in a Satisfies or Narrows row"})
		case !strong[n.id] && l.isDefined(n.id):
			findings = append(findings, finding{path: n.path, line: n.line, message: n.id + " in the table of standard clauses not applied is not a strong norm of a GATES role"})
		}
	}
	for _, d := range l.definitions {
		if strong[d.id] && !related[d.id] && !notApplied[d.id] {
			findings = append(findings, finding{
				path:    d.path,
				line:    d.line,
				message: fmt.Sprintf("%s is a strong norm of a GATES role, but no Satisfies or Narrows row and no table of standard clauses not applied lists it", d.id),
			})
		}
	}
	return findings
}

func (l *links) isDefined(id string) bool {
	return slices.ContainsFunc(l.definitions, func(d definition) bool { return d.id == id })
}

// formalizationFindings reports a clause or a GOJ that a model neither
// formalizes nor marks unformalized, or does both. An Unconfirmed GitHub
// clause takes neither, because a model leaves what GitHub has not settled
// unconstrained.
func (l *links) formalizationFindings() []finding {
	formalized := make(map[string]bool)
	for _, a := range l.annotations {
		if a.kind == formalizes && a.inModel() {
			for _, id := range a.ids {
				formalized[id] = true
			}
		}
	}
	unformalizedIDs := l.unformalizedIDs()
	var findings []finding
	report := func(d definition, message string) {
		findings = append(findings, finding{path: d.path, line: d.line, message: d.id + " " + message})
	}
	for _, d := range l.definitions {
		if !isClause(d.id) && !isGOJ(d.id) {
			continue
		}
		f, u := formalized[d.id], unformalizedIDs[d.id]
		switch {
		case isGitHubClause(d.id) && d.fields["Evidence"] == "Unconfirmed":
			if f || u {
				report(d, "is Unconfirmed, so it takes neither trace:formalizes nor trace:unformalized")
			}
		case f && u:
			report(d, "has both trace:formalizes and trace:unformalized")
		case !f && !u:
			report(d, "has neither trace:formalizes nor trace:unformalized")
		}
	}
	return findings
}
