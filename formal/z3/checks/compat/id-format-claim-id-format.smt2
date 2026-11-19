; expect: sat sat
; trace:checks GOJ-ID-FORMAT GH-OIDC-CLAIM-ID-FORMAT
; Nothing fixes the spelling of an ID, so an issuance whose repository_id and
; repository ID in sub have a leading zero is allowed, and so is one whose
; repository_id and repository ID in sub have a character other than a digit.
; GATES rejects both. The fixed names and IDs only guide the search.
(push 1)
(assert documented-issuance)
(assert immutable-repository)
(assert (= owner "octo-org"))
(assert (= ownerId "123456"))
(assert (= repo "octo-repo"))
(assert (= repositoryId "074"))
(assert (= (select issued "repository_id") (Present (JString repositoryId))))
(assert (not (gates-id-claim issued "repository_id")))
(check-sat)
(pop 1)
(push 1)
(assert documented-issuance)
(assert immutable-repository)
(assert (= owner "octo-org"))
(assert (= ownerId "123456"))
(assert (= repo "octo-repo"))
(assert (= repositoryId "R_kgDO1"))
(assert (= (select issued "repository_id") (Present (JString repositoryId))))
(assert (not (gates-id-claim issued "repository_id")))
(check-sat)
(pop 1)
