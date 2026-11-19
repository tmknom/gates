package main

import (
	"fmt"
	"io"
	"slices"
	"strings"
)

// writeMap writes, for every clause and GOJ, the annotations that formalize it,
// mark it unformalized, and check it. The documents do not name predicates or
// checks, so this table is the way to look up a definition's models; it is
// generated each time and not committed.
func writeMap(w io.Writer, l *links) error {
	var ids []string
	for _, d := range l.definitions {
		if (isClause(d.id) || isGOJ(d.id)) && !slices.Contains(ids, d.id) {
			ids = append(ids, d.id)
		}
	}
	slices.Sort(ids)
	locations := func(id string, kind annotationKind) string {
		var found []string
		for _, a := range l.annotations {
			if a.kind == kind && slices.Contains(a.ids, id) {
				found = append(found, fmt.Sprintf("%s:%d", a.path, a.line))
			}
		}
		slices.Sort(found)
		return strings.Join(found, "<br>")
	}
	if _, err := fmt.Fprintln(w, "| ID | Formalized by | Unformalized at | Checked by |\n|---|---|---|---|"); err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := fmt.Fprintf(w, "| %s | %s | %s | %s |\n", id, locations(id, formalizes), locations(id, unformalized), locations(id, checks)); err != nil {
			return err
		}
	}
	return nil
}
