; expect: unsat
; trace:checks GOJ-HEADER-CTY GH-OIDC-HEADER-OBSERVED
; Every recorded Header consists of alg, kid, typ and x5t alone, so it has no
; cty, which GATES accepts.
(assert observed-issuance)
(assert (not (gates-header-cty issuedHeader)))
(check-sat)
