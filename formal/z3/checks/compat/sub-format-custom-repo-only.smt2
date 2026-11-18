; expect: sat
; trace:checks GOJ-SUB-FORMAT GH-OIDC-SUB-CUSTOM-REPO-ONLY
; A template of repo alone, in a repository with the immutable form, yields a
; sub that holds the names and IDs; nothing fixes what follows them, so a sub
; that ends at the repository ID is allowed, which GATES rejects.
; The fixed names and IDs only guide the search.
(assert documented-issuance)
(assert documented-names)
(assert customized)
(assert (gh-sub-custom-immutable-ids issued form true true owner ownerId repo repositoryId))
(assert (= (select issued "sub")
           (Present (JString (str.++ "repo:" owner "@" ownerId "/" repo "@" repositoryId)))))
(assert (not (gates-sub-format issued)))
(check-sat)
