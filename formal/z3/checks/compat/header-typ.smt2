; expect: unsat unsat
; trace:checks GOJ-HEADER-TYP GH-OIDC-HEADER-DOCUMENTED GH-OIDC-HEADER-OBSERVED
; GATES reads neither the presence nor the value of typ, so it accepts the typ
; that the clauses describe, JWT in the records, and any other. The first
; check-sat shows typ is not consulted; the second that typ of any value or
; absent leaves acceptance as it is.
(declare-const member Member)
(push 1)
(assert (header-consulted-name "typ"))
(check-sat)
(pop 1)
(push 1)
(assert (not (= (gates-header-accepted (header-with-member issuedHeader "typ" member))
                (gates-header-accepted (header-with-member issuedHeader "typ" Absent)))))
(check-sat)
(pop 1)
