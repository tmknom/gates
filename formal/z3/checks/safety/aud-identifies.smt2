; expect: unsat
; trace:checks GOJ-AUD RFC7519-AUD-INTENDED RFC7519-AUD-REJECT
; Every Claims Set GATES accepts for an Audience identifies that Audience in aud.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(assert (gates-current anyClaims arbitraryClock arbitraryAudience))
(assert (not (std-aud-identifies anyClaims arbitraryAudience)))
(check-sat)
