; expect: unsat
; trace:checks GOJ-SUB-LENGTH OIDCC-SUB-LENGTH
; GOJ-SUB-LENGTH narrows RFC7519-SUB-FORMAT to the limit OIDCC-SUB-LENGTH puts
; on the sub of an ID Token. Every sub GATES accepts is within that limit, as
; if the JWT were an ID Token.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(assert (gates-claims-accepted anyClaims arbitraryClock arbitraryAudience))
(assert (not (std-oidc-sub-length anyClaims true)))
(check-sat)
