; expect: sat
; trace:checks GOJ-HEADER-CRIT GH-OIDC-HEADER-PARAMETERS
; Nothing fixes whether GitHub adds crit to the Header, which GATES rejects.
(assert documented-issuance)
(assert ((_ is Present) (select issuedHeader "crit")))
(assert (not (gates-header-crit issuedHeader)))
(check-sat)
