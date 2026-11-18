; expect: sat
; trace:checks GOJ-IAT GH-OIDC-CLAIM-TYPE
; The Documented clauses allow an issuance whose iat is a JSON number that is
; not an integer, which GATES rejects.
(assert documented-issuance)
(assert (= (select issued "iat") (Present (JFraction (- (to_real usedAt) 0.5)))))
(assert (not (gates-iat issued)))
(check-sat)
