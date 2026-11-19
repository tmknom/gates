; expect: sat
; trace:checks GOJ-SUB-FORMAT GH-OIDC-SUB-CUSTOM-WITHOUT-REPO
; Nothing fixes the sub of a template without repo in a repository with the
; immutable form, so a sub without the repo part is allowed, which GATES
; rejects.
; The fixed names and IDs only guide the search.
(assert documented-issuance)
(assert documented-names)
(assert customized)
(assert (gh-sub-custom-immutable-ids issued form true false owner ownerId repo repositoryId))
(assert (= (select issued "sub") (Present (JString (str.++ "repository_owner:" owner)))))
(assert (not (gates-sub-format issued)))
(check-sat)
