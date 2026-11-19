; expect: sat
; trace:checks GOJ-RUN-ATTEMPT GH-OIDC-CLAIM-PRESENCE
; The Documented clauses allow an issuance without run_attempt, which GATES
; rejects.
(assert documented-issuance)
(assert (= (select issued "run_attempt") Absent))
(assert (not (gates-run-attempt issued)))
(check-sat)
