; expect: unsat
; trace:checks GOJ-HEADER-CRIT GH-OIDC-HEADER-OBSERVED
; Every recorded Header consists of alg, kid, typ and x5t alone, so it has no
; crit, which GATES accepts.
(assert observed-issuance)
(assert (not (gates-header-crit issuedHeader)))
(check-sat)
