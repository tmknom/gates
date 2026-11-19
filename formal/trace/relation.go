package main

import (
	"fmt"
	"slices"
)

// relationTableFindings reports a GOJ without a relation table and a relation
// row that breaks the format of the Traceability Policy.
func (l *links) relationTableFindings() []finding {
	rows := make(map[string][]relation)
	for _, r := range l.relations {
		rows[r.definition] = append(rows[r.definition], r)
	}
	var findings []finding
	report := func(r relation, format string, args ...any) {
		findings = append(findings, finding{path: r.path, line: r.line, message: fmt.Sprintf(format, args...)})
	}
	for _, d := range l.definitions {
		if isGOJ(d.id) && len(rows[d.id]) == 0 {
			findings = append(findings, finding{path: d.path, line: d.line, message: d.id + " has no relation table"})
		}
	}
	for _, r := range l.relations {
		if r.note == "" {
			report(r, "relation row has an empty Note")
		}
		switch {
		case r.kind == none:
			if r.clause != noClause {
				report(r, "None row has Clause %q, want %q", r.clause, noClause)
			}
			if len(rows[r.definition]) > 1 {
				report(r, "None row is not the only relation row of %s", r.definition)
			}
		case !isID(r.clause):
			report(r, "Clause %q is not an ID", r.clause)
		case (r.kind == satisfies || r.kind == narrows) && !isStandardClause(r.clause):
			report(r, "%s row has %s, want a standard clause", r.kind, r.clause)
		case (r.kind == accepts || r.kind == possiblyRejects) && !isGitHubClause(r.clause):
			report(r, "%s row has %s, want a GitHub clause", r.kind, r.clause)
		}
		if r.kind == possiblyRejects && slices.ContainsFunc(rows[r.definition], func(o relation) bool { return o.kind == accepts && o.clause == r.clause }) {
			report(r, "%s both Accepts and Possibly Rejects %s", r.definition, r.clause)
		}
	}
	return findings
}

// uncheckedRelations reports a relation row without the check the Traceability
// Policy requires. A Satisfies relation needs a check that finds no
// counterexample, so that every JWT the GOJ accepts is shown to satisfy the
// clause. An Accepts relation needs a formal check, and a Possibly Rejects
// relation a check that shows an issuance the GOJ rejects. When the GOJ or the
// clause of a Satisfies or Accepts relation is not formalized, because the
// model holds decoded values and not the bytes of the JWT, a Go test may be
// the check instead.
func (l *links) uncheckedRelations() []finding {
	unformalizedIDs := l.unformalizedIDs()
	var findings []finding
	report := func(r relation, on string) {
		findings = append(findings, finding{
			path:    r.path,
			line:    r.line,
			message: fmt.Sprintf("%s %s %s has no trace:checks listing both IDs on %s", r.kind, r.definition, r.clause, on),
		})
	}
	for _, r := range l.relations {
		if !isID(r.clause) {
			continue
		}
		switch r.kind {
		case satisfies, accepts:
			formal, on := annotation.inModel, "a formal check"
			if r.kind == satisfies {
				formal, on = func(a annotation) bool { return a.inModel() && a.universal }, "a check expecting no counterexample"
			}
			switch {
			case l.checkedBy(r, formal):
			case !unformalizedIDs[r.definition] && !unformalizedIDs[r.clause]:
				report(r, on)
			case !l.checkedBy(r, annotation.inGoTest):
				report(r, "a Go test or "+on)
			}
		case possiblyRejects:
			if !l.checkedBy(r, func(a annotation) bool { return a.inModel() && a.witness }) {
				report(r, "a check expecting an instance")
			}
		}
	}
	return findings
}

func (l *links) checkedBy(r relation, accept func(annotation) bool) bool {
	return slices.ContainsFunc(l.annotations, func(a annotation) bool {
		return a.kind == checks && accept(a) && a.lists(r.definition, r.clause)
	})
}

// unformalizedIDs returns the IDs that a model marks with trace:unformalized.
func (l *links) unformalizedIDs() map[string]bool {
	ids := make(map[string]bool)
	for _, a := range l.annotations {
		if a.kind == unformalized && a.inModel() {
			for _, id := range a.ids {
				ids[id] = true
			}
		}
	}
	return ids
}

// unrelatedChecks reports a trace:checks that lists a GOJ and a clause that no
// relation row links, so that deleting a row leaves no check behind that
// still claims the relation. A clause named in the Note of a row of the GOJ
// counts as linked. A check that expects an instance shows an issuance the
// GOJ rejects, so its GitHub clause must be in a Possibly Rejects row.
func (l *links) unrelatedChecks() []finding {
	var findings []finding
	for _, a := range l.annotations {
		if a.kind != checks {
			continue
		}
		for _, goj := range a.ids {
			if !isGOJ(goj) {
				continue
			}
			for _, clause := range a.ids {
				if !isClause(clause) {
					continue
				}
				switch {
				case !l.linked(goj, clause):
					findings = append(findings, finding{path: a.path, line: a.line, message: fmt.Sprintf("trace:checks lists %s and %s, which no relation row of %s links", goj, clause, goj)})
				case a.witness && isGitHubClause(clause) && !l.hasRelation(goj, possiblyRejects, clause):
					findings = append(findings, finding{path: a.path, line: a.line, message: fmt.Sprintf("trace:checks expects an instance for %s and %s, which is not a Possibly Rejects relation", goj, clause)})
				}
			}
		}
	}
	return findings
}

func (l *links) linked(goj, clause string) bool {
	return slices.ContainsFunc(l.relations, func(r relation) bool {
		return r.definition == goj && (r.clause == clause || slices.Contains(findIDs(r.note), clause))
	})
}

func (l *links) hasRelation(goj, kind, clause string) bool {
	return slices.ContainsFunc(l.relations, func(r relation) bool {
		return r.definition == goj && r.kind == kind && r.clause == clause
	})
}
