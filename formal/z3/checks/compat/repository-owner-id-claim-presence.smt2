; expect: sat
; trace:checks GOJ-REPOSITORY-OWNER-ID GH-OIDC-CLAIM-PRESENCE
; The Documented clauses allow an issuance without repository_owner_id, which GATES rejects.
(assert documented-issuance)
(assert (= (select issued "repository_owner_id") Absent))
(assert (not (gates-repository-owner-id issued)))
(check-sat)
