; expect: sat
; trace:checks GOJ-HEADER-B64 GH-OIDC-HEADER-PARAMETERS
; Nothing fixes whether GitHub adds b64 to the Header, which GATES rejects.
(assert documented-issuance)
(assert ((_ is Present) (select issuedHeader "b64")))
(assert (not (gates-header-b64 issuedHeader)))
(check-sat)
