; expect: sat
; trace:checks GOJ-IAT GH-OIDC-CLAIM-PRESENCE
; The Documented clauses allow an issuance without iat, which GATES rejects.
(assert documented-issuance)
(assert (= (select issued "iat") Absent))
(assert (not (gates-iat issued)))
(check-sat)
