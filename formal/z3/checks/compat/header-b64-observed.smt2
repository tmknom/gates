; expect: unsat
; trace:checks GOJ-HEADER-B64 GH-OIDC-HEADER-OBSERVED
; Every recorded Header consists of alg, kid, typ and x5t alone, so it has no
; b64, which GATES accepts.
(assert observed-issuance)
(assert (not (gates-header-b64 issuedHeader)))
(check-sat)
