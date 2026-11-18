; expect: sat
; trace:checks GOJ-HEADER-UNKNOWN GH-OIDC-HEADER-PARAMETERS
; Nothing fixes whether GitHub adds nonce to the Header. A nonce that is not a
; string is allowed, which GATES rejects.
(assert documented-issuance)
(assert (= (select issuedHeader "nonce") (Present (JInteger 1))))
(assert (not (gates-header-unknown issuedHeader)))
(check-sat)
