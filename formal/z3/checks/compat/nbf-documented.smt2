; expect: sat
; trace:checks GOJ-NBF
; The Documented clauses bound nbf by neither a type nor a range. Even for a
; present integer nbf (GH-OIDC-CLAIM-PRESENCE, GH-OIDC-CLAIM-TYPE), they allow
; one outside 0..253402300799, which GATES rejects: GATES relies on the
; Observed clauses to accept nbf.
(assert documented-issuance)
(assert clocks-in-range)
(assert (=> ((_ is Present) (select issued "nbf")) ((_ is JInteger) (member-value (select issued "nbf")))))
(assert (not (gates-nbf issued)))
(check-sat)
