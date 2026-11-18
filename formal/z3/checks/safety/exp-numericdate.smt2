; expect: unsat
; trace:checks GOJ-EXP RFC7519-EXP-NUMERICDATE
; Every Claims Set GATES accepts has an exp that is a NumericDate.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(assert (gates-current anyClaims arbitraryClock arbitraryAudience))
(assert (not (std-exp-numericdate anyClaims)))
(check-sat)
