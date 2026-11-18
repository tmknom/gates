; expect: unsat
; trace:checks GOJ-REPOSITORY-OWNER-ID GH-OIDC-REPOSITORY-OWNER-ID
; GATES accepts the repository_owner_id of every issuance the Documented clauses allow,
; once it is present (GH-OIDC-CLAIM-PRESENCE), a string (GH-OIDC-CLAIM-TYPE)
; and spelled as GOJ-ID-FORMAT asks (GH-OIDC-CLAIM-ID-FORMAT).
(assert documented-issuance)
(assert (string-member issued "repository_owner_id"))
(assert (gates-id-text ownerId))
(assert (not (gates-repository-owner-id issued)))
(check-sat)
