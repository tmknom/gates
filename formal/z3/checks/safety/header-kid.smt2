; expect: unsat
; trace:checks GOJ-HEADER-KID RFC7515-KID-FORMAT
; Every JOSE Header GATES accepts has a kid that is a string.
(declare-const anyHeader Header)
(assert (gates-header-accepted anyHeader))
(assert (not (std-jws-kid-format anyHeader)))
(check-sat)
