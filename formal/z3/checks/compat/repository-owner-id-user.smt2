; expect: sat
; trace:checks GOJ-REPOSITORY-OWNER-ID GH-OIDC-REPOSITORY-OWNER-ID-USER
; Nothing says that the JWT of a repository a User owns has
; repository_owner_id, and the Documented clause describes the ID of an
; Organization. An issuance without it is allowed, which GATES rejects.
(assert documented-issuance)
(assert (= (select issued "repository_owner_id") Absent))
(assert (not (gates-repository-owner-id issued)))
(check-sat)
