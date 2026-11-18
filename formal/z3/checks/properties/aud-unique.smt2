; expect: unsat
; trace:checks GOJ-AUD
; A Claims Set accepted for one expected Audience is not accepted for any
; other, so a JWT is a credential for one receiver only.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const audience String)
(declare-const otherAudience String)
(assert (gates-claims-accepted anyClaims arbitraryClock audience))
(assert (gates-claims-accepted anyClaims arbitraryClock otherAudience))
(assert (not (= audience otherAudience)))
(check-sat)
