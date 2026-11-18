; expect: sat
; trace:checks GOJ-AUD GH-OIDC-CLAIM-PRESENCE
; The Documented clauses allow an issuance without aud, which GATES rejects.
(assert documented-issuance)
(assert (= (select issued "aud") Absent))
(assert (not (gates-aud issued expected)))
(check-sat)
