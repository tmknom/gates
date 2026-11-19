; expect: sat
; trace:checks GOJ-HEADER-CTY GH-OIDC-HEADER-PARAMETERS
; Nothing fixes whether GitHub adds cty to the Header, which GATES rejects.
(assert documented-issuance)
(assert ((_ is Present) (select issuedHeader "cty")))
(assert (not (gates-header-cty issuedHeader)))
(check-sat)
