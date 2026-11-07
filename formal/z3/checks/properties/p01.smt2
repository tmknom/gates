; expect: sat
; P01: an accepted Claims Set exists, so the universal checks do not hold
; vacuously under contradictory assumptions.
(assert (gates-current sample 1000 "gates-oidc"))
(check-sat)
