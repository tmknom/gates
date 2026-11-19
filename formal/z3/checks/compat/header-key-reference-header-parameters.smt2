; expect: sat
; trace:checks GOJ-HEADER-KEY-REFERENCE GH-OIDC-HEADER-PARAMETERS
; Nothing fixes whether GitHub adds jwk to the Header. A jwk that does not
; read as a JWK is allowed, which GATES rejects.
(assert documented-issuance)
(assert (= (select issuedHeader "jwk") (Present (JString "key"))))
(assert (not (gates-header-key-reference issuedHeader)))
(check-sat)
