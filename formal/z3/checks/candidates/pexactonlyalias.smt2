; expect: unsat
; PEXACTONLYALIAS: the exact-name candidate and the current design differ
; only on sets with an alias present.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(assert (gates-exact-name-candidate anyClaims arbitraryClock arbitraryAudience))
(assert (aliases-absent anyClaims))
(assert (not (gates-current anyClaims arbitraryClock arbitraryAudience)))
(check-sat)
