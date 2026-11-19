; expect: sat
; trace:checks GOJ-REPOSITORY-ID GH-OIDC-CLAIM-PRESENCE
; The Documented clauses allow an issuance without repository_id, which GATES rejects.
(assert documented-issuance)
(assert (= (select issued "repository_id") Absent))
(assert (not (gates-repository-id issued)))
(check-sat)
