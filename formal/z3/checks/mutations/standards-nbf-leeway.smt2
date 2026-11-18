; expect: sat
; checks/safety/time-nbf-reject.smt2 finds a counterexample when the nbf
; leeway clause is misread as bad-std-nbf-leeway, which allows no leeway: the
; 300 seconds GATES takes falls outside it. Tightening the clause cannot let
; the check pass vacuously.
(assert (gates-claims-accepted sample 1000 "gates-oidc"))
(assert (not (bad-std-nbf-leeway 300.0)))
(check-sat)
