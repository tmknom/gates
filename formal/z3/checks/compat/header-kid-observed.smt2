; expect: unsat
; trace:checks GOJ-HEADER-KID GH-OIDC-HEADER-DOCUMENTED GH-OIDC-HEADER-OBSERVED GH-OIDC-HEADER-KID-JWKS GH-OIDC-JWKS-KID-FORMAT
; In every issuance the Published and Observed clauses allow, kid is present
; and equals the kid of a JWKS key, which is a UUID or 40 hexadecimal digits,
; so GATES accepts it.
(assert observed-issuance)
(assert (not (gates-header-kid issuedHeader)))
(check-sat)
