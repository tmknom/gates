; expect: unsat
; trace:checks GOJ-TIME-NBF RFC7519-NBF-REJECT RFC7519-NBF-LEEWAY
; GATES accepts no JWT before its nbf, beyond the leeway of 300 seconds that
; GATES takes under RFC7519-NBF-LEEWAY. A leeway that clause does not allow
; is a counterexample too, so tightening std-nbf-leeway cannot pass vacuously.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(assert (gates-current anyClaims arbitraryClock arbitraryAudience))
(assert (or (not (std-nbf-leeway 300.0))
            (not (std-nbf-reject anyClaims arbitraryClock 300.0))))
(check-sat)
