; expect: unsat
; PAUDUNIQUE (jwt.md §2.3): a Claims Set accepted for one expected Audience is
; not accepted for any other, so a JWT is a credential for one receiver only.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const audience String)
(declare-const otherAudience String)
(assert (gates-current anyClaims arbitraryClock audience))
(assert (gates-current anyClaims arbitraryClock otherAudience))
(assert (not (= audience otherAudience)))
(check-sat)
