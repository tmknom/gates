; expect: sat
; trace:checks GOJ-ISS GH-OIDC-CLAIM-PRESENCE
; The Documented clauses allow an issuance without iss, which GATES rejects.
(assert documented-issuance)
(assert (= (select issued "iss") Absent))
(assert (not (gates-iss issued)))
(check-sat)
