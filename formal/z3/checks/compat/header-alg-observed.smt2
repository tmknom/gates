; expect: unsat
; trace:checks GOJ-HEADER-ALG GH-OIDC-DISCOVERY-ALG GH-OIDC-HEADER-OBSERVED
; In every issuance the Published and Observed clauses allow, alg is RS256,
; which GATES accepts.
(assert observed-issuance)
(assert (not (gates-header-alg issuedHeader)))
(check-sat)
