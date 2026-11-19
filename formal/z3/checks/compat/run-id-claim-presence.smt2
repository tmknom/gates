; expect: sat
; trace:checks GOJ-RUN-ID GH-OIDC-CLAIM-PRESENCE
; The Documented clauses allow an issuance without run_id, which GATES rejects.
(assert documented-issuance)
(assert (= (select issued "run_id") Absent))
(assert (not (gates-run-id issued)))
(check-sat)
