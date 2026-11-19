; expect: unsat
; trace:checks GOJ-REPOSITORY-OWNER-ID GH-OIDC-REPOSITORY-OWNER-ID-OBSERVED
; In every issuance the Observed clauses allow, repository_owner_id is present
; and a string of digits, so GATES accepts it once the digits have the
; GOJ-ID-FORMAT spelling.
(assert observed-issuance)
(assert (gates-id-text ownerId))
(assert (not (gates-repository-owner-id issued)))
(check-sat)
