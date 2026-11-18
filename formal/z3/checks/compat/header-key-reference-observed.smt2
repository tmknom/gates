; expect: unsat
; trace:checks GOJ-HEADER-KEY-REFERENCE GH-OIDC-HEADER-OBSERVED
; Every recorded Header has x5t and neither jwk nor x5c, which GATES accepts.
(assert observed-issuance)
(assert (not (gates-header-key-reference issuedHeader)))
(check-sat)
