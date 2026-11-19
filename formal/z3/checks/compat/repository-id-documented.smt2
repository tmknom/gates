; expect: unsat
; trace:checks GOJ-REPOSITORY-ID GH-OIDC-REPOSITORY-ID
; GATES accepts the repository_id of every issuance the Documented clauses allow,
; once it is present (GH-OIDC-CLAIM-PRESENCE), a string (GH-OIDC-CLAIM-TYPE)
; and spelled as GOJ-ID-FORMAT asks (GH-OIDC-CLAIM-ID-FORMAT).
(assert documented-issuance)
(assert (string-member issued "repository_id"))
(assert (gates-id-text repositoryId))
(assert (not (gates-repository-id issued)))
(check-sat)
