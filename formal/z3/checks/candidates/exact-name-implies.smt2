; expect: unsat
; Every set the current design accepts, the exact-name
; candidate also accepts.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(assert (gates-claims-accepted anyClaims arbitraryClock arbitraryAudience))
(assert (not (gates-exact-name-candidate anyClaims arbitraryClock arbitraryAudience)))
(check-sat)
