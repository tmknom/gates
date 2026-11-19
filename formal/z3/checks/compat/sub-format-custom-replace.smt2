; expect: sat
; trace:checks GOJ-SUB-FORMAT GH-OIDC-SUB-CUSTOM-REPLACE
; A template replaces the whole default form, so a sub that does not start
; with repo: is allowed, which GATES rejects.
(assert documented-issuance)
(assert customized)
(assert (string-member issued "sub"))
(assert (not (str.prefixof "repo:" (string-member-value issued "sub"))))
(assert (not (gates-sub-format issued)))
(check-sat)
