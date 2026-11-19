; expect: sat
; trace:checks GOJ-SUB-LENGTH GH-OIDC-SUB-LENGTH
; Nothing fixes the length or the characters of sub, so the immutable form of
; a context with a character outside ASCII is allowed, which GATES rejects.
; The fixed names and IDs only guide the search.
(assert documented-issuance)
(assert immutable-repository)
(assert documented-names)
(assert (= context "environment:\u{e9}"))
(assert ((_ is Present) (select issued "sub")))
(assert (not (gates-sub-length issued)))
(check-sat)
