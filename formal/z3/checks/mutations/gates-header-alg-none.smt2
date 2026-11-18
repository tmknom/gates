; expect: sat
; checks/safety/header-none.smt2 finds a counterexample against
; bad-header-alg-none.
(declare-const anyHeader Header)
(assert (bad-header-alg-none anyHeader))
(assert (not (std-none-accept-default anyHeader false)))
(check-sat)
