; expect: sat
; trace:checks GOJ-SUB-FORMAT GH-OIDC-SUB-LEGACY-RETAINED
; A repository created before the date that has not opted in, been renamed or
; transferred, or taken a template keeps the legacy form, whose sub GATES
; rejects.
; The fixed names and IDs only guide the search.
(assert documented-issuance)
(assert documented-names)
(assert (= context "ref:refs/heads/main"))
(assert (not createdAfter))
(assert (not optedIn))
(assert (not renamed))
(assert (not customized))
(assert ((_ is Present) (select issued "sub")))
(assert (not (gates-sub-format issued)))
(check-sat)
