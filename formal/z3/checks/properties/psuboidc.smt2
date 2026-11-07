; expect: unsat
; PSUBOIDC: every GATES-accepted sub satisfies OIDC Core §2 ASCII/255.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(assert (gates-current anyClaims arbitraryClock arbitraryAudience))
(assert (not (oidc-sub-conditional anyClaims)))
(check-sat)
