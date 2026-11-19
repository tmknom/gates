; expect: sat
; trace:checks GOJ-SUB-FORMAT GH-OIDC-CLAIM-PRESENCE
; The Documented clauses allow an issuance without sub, which GATES rejects.
(assert documented-issuance)
(assert (= (select issued "sub") Absent))
(assert (not (gates-sub-format issued)))
(check-sat)
