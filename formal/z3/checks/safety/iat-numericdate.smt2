; expect: unsat
; trace:checks GOJ-IAT RFC7519-IAT-NUMERICDATE
; Every Claims Set GATES accepts has an iat that is a NumericDate.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(assert (gates-current anyClaims arbitraryClock arbitraryAudience))
(assert (not (std-iat-numericdate anyClaims)))
(check-sat)
