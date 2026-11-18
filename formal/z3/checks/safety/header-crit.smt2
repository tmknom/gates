; expect: unsat
; trace:checks GOJ-HEADER-CRIT RFC7515-CRIT-UNDERSTAND RFC7515-CRIT-UNSUPPORTED-INVALID
; GATES understands no extension. Every JOSE Header GATES accepts lists no
; extension in crit that it does not understand.
(declare-const anyHeader Header)
(assert (gates-header-accepted anyHeader))
(assert (not (std-jws-crit-understood anyHeader ((as const (Array String Bool)) false))))
(check-sat)
