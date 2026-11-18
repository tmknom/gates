; expect: unsat
; The exact-name candidate and the current design differ
; only on sets with an alias present.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(assert (gates-exact-name-candidate anyClaims arbitraryClock arbitraryAudience))
(assert (gates-aliases-absent anyClaims))
(assert (not (gates-claims-accepted anyClaims arbitraryClock arbitraryAudience)))
(check-sat)
