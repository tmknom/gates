; expect: sat
; trace:checks GOJ-EXP GH-OIDC-CLAIM-PRESENCE
; The Documented clauses allow an issuance without exp, which GATES rejects.
(assert documented-issuance)
(assert (= (select issued "exp") Absent))
(assert (not (gates-exp issued)))
(check-sat)
