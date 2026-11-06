; P02 (universal): for arbitrary decoded Claims, clock and audience,
; a GATES-accepted set cannot violate the SELECTED RFC receiver obligations.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(assert (gates-current-example-aliases anyClaims arbitraryClock arbitraryAudience))
(assert (not (standard-selected anyClaims arbitraryClock arbitraryAudience)))
(check-sat)
