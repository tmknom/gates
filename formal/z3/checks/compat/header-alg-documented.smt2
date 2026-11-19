; expect: sat
; trace:checks GOJ-HEADER-ALG
; No Documented clause fixes alg: the official example shows RS256 for one
; JWT (GH-OIDC-HEADER-EXAMPLE). A Header with another alg is allowed, which
; GATES rejects: GATES relies on the Published and Observed clauses to accept
; alg.
(assert documented-issuance)
(assert (string-member issuedHeader "alg"))
(assert (not (gates-header-alg issuedHeader)))
(check-sat)
