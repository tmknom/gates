; expect: sat
; trace:checks GOJ-SUB-FORMAT GH-OIDC-SUB-LENGTH
; Nothing fixes the characters of sub, so the immutable form of an owner name
; with '/' is allowed, which GATES rejects because the owner part ends at the
; first '/'.
; The fixed names and IDs only guide the search.
(assert documented-issuance)
(assert immutable-repository)
(assert (= owner "octo/org"))
(assert (= ownerId "123456"))
(assert (= repo "octo-repo"))
(assert (= repositoryId "456789"))
(assert (= context "ref:refs/heads/main"))
(assert ((_ is Present) (select issued "sub")))
(assert (not (gates-sub-format issued)))
(check-sat)
