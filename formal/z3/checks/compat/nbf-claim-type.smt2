; expect: sat
; trace:checks GOJ-NBF GH-OIDC-CLAIM-TYPE
; The Documented clauses allow an issuance whose nbf is a JSON number that is
; not an integer, which GATES rejects.
(assert documented-issuance)
(assert (= (select issued "nbf") (Present (JFraction (- (to_real usedAt) 0.5)))))
(assert (not (gates-nbf issued)))
(check-sat)
