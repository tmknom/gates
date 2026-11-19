; expect: unsat
; trace:checks GOJ-EXP GOJ-TIME-EXP
; No Claims Set is accepted at or after 253402300799, the upper limit of exp,
; so every accepted JWT expires.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(assert (gates-claims-accepted anyClaims arbitraryClock arbitraryAudience))
(assert (>= arbitraryClock 253402300799))
(check-sat)
