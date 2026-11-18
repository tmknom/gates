; expect: sat
; trace:checks GOJ-ID-FORMAT GH-OIDC-CLAIM-TYPE
; The Documented clauses allow an issuance whose repository_id is a JSON
; number, which GATES rejects.
(assert documented-issuance)
(assert (= (select issued "repository_id") (Present (JInteger 74))))
(assert (not (gates-id-claim issued "repository_id")))
(check-sat)
