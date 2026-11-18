; expect: sat
; trace:checks GOJ-EXP GH-OIDC-CLAIM-TYPE
; The Documented clauses allow an issuance whose exp is a JSON number that is
; not an integer, which GATES rejects.
(assert documented-issuance)
(assert (= (select issued "exp") (Present (JFraction (+ (to_real usedAt) 0.5)))))
(assert (not (gates-exp issued)))
(check-sat)
