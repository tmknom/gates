; expect: sat
; checks/safety/header-defined-ignore.smt2 finds a counterexample against
; bad-header-typ-required: typ, which GATES does not consult, changes
; acceptance.
(declare-const value JsonValue)
(assert (not (std-jws-defined-unknown-ignore
               "typ" (header-consulted-name "typ")
               (bad-header-typ-required (store sample-header "typ" (Present value)))
               (bad-header-typ-required (store sample-header "typ" Absent)))))
(check-sat)
