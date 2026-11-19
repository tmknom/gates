; expect: unsat
; trace:checks GOJ-HEADER-ALG RFC7518-NONE-ACCEPT-DEFAULT RFC7518-NONE-GLOBAL RFC8725-ALG-NONE-CONSUME
; GATES is not configured to accept an Unsecured JWS, indicates acceptance
; for no object, and never asks the library for none. Every JOSE Header GATES
; accepts has an alg other than none. GATES accepts every Header the same way,
; so accepting one Header with none would signal acceptance at a global level.
(declare-const anyHeader Header)
(assert (gates-header-accepted anyHeader))
(assert (not (and (std-none-accept-default anyHeader false)
                  (std-none-global (alg-none anyHeader))
                  (std-alg-none-consume anyHeader false))))
(check-sat)
