; expect: sat
; trace:checks GOJ-EXP
; The Documented clauses bound exp by neither a type nor a range. Even for a
; present integer exp (GH-OIDC-CLAIM-PRESENCE, GH-OIDC-CLAIM-TYPE), they allow
; one outside 0..253402300799, which GATES rejects: GATES relies on the
; Observed clauses to accept exp.
(assert documented-issuance)
(assert clocks-in-range)
(assert ((_ is Present) (select issued "exp")))
(assert ((_ is JInteger) (member-value (select issued "exp"))))
(assert (not (gates-exp issued)))
(check-sat)
