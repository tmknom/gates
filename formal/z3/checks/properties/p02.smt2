; expect: unsat
; P02: for arbitrary decoded Claims, clock and audience, a GATES-accepted set
; cannot violate the SELECTED RFC receiver obligations on iss and sub.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(assert (gates-current anyClaims arbitraryClock arbitraryAudience))
(assert (not (standard-selected anyClaims)))
(check-sat)
