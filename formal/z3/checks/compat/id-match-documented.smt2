; expect: unsat
; trace:checks GOJ-ID-MATCH GH-OIDC-SUB-IMMUTABLE-FORMAT
; In every issuance the Documented clauses allow for a repository whose
; default sub has the immutable form, the IDs in sub are those of
; repository_owner_id (GH-OIDC-REPOSITORY-OWNER-ID) and repository_id
; (GH-OIDC-REPOSITORY-ID). GATES finds them equal once the Claims are present
; strings (GH-OIDC-CLAIM-PRESENCE, GH-OIDC-CLAIM-TYPE). The names, IDs and
; context are fixed for the reason in sub-format-documented.smt2.
(assert documented-issuance)
(assert immutable-repository)
(assert documented-names)
(assert (= context "ref:refs/heads/main"))
(assert ((_ is Present) (select issued "sub")))
(assert (string-member issued "repository_owner_id"))
(assert (string-member issued "repository_id"))
(assert (not (gates-id-match issued)))
(check-sat)
