; expect: unsat
; trace:checks GOJ-HEADER-B64 RFC7797-JWT-B64-FALSE
; Every JOSE Header GATES accepts has no b64 of false.
(declare-const anyHeader Header)
(assert (gates-header-accepted anyHeader))
(assert (not (std-jwt-b64-not-false anyHeader)))
(check-sat)
