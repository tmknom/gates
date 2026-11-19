; expect: unsat
; trace:checks GOJ-NBF RFC7519-NBF-NUMERICDATE
; Every Claims Set GATES accepts has no nbf or an nbf that is a NumericDate.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(assert (gates-claims-accepted anyClaims arbitraryClock arbitraryAudience))
(assert (not (std-nbf-numericdate anyClaims)))
(check-sat)
