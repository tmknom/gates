; expect: unsat
; trace:checks GOJ-TIME-EXP RFC7519-EXP-REJECT
; GATES accepts no JWT on or after its exp, even without the leeway that
; RFC7519-EXP-LEEWAY allows, so the leeway is 0 here.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(assert (gates-current anyClaims arbitraryClock arbitraryAudience))
(assert (not (std-exp-reject anyClaims arbitraryClock 0.0)))
(check-sat)
