; expect: unsat
; PAUDITINV (GOJ-TRIGGER-ATTRIBUTES): none of the eight audit names is consulted, so
; by PCONGRUENCE their values, types or absence cannot change acceptance.
(declare-const anyName String)
(assert (audit-name anyName))
(assert (consulted-name anyName))
(check-sat)
