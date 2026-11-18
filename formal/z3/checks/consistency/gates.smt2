; expect: sat
; trace:checks GOJ-EXP GOJ-IAT GOJ-NBF GOJ-TIME GOJ-AUD
; An accepted Claims Set exists, so the GATES acceptance predicates do not
; contradict each other and the universal checks do not hold vacuously.
(assert (gates-current sample 1000 "gates-oidc"))
(check-sat)
