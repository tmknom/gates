; expect: sat sat
; trace:checks RFC7519-NUMERICDATE-FORMAT RFC7519-EXP-NUMERICDATE RFC7519-EXP-OPTIONAL RFC7519-EXP-LEEWAY RFC7519-EXP-REJECT RFC7519-NBF-NUMERICDATE RFC7519-NBF-OPTIONAL RFC7519-NBF-LEEWAY RFC7519-NBF-REJECT RFC7519-IAT-NUMERICDATE RFC7519-IAT-OPTIONAL
; The time clauses do not contradict each other: some Claims Set satisfies all
; of them with exp, nbf and iat present, and some with all three omitted.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const expLeeway Real)
(declare-const nbfLeeway Real)
(define-fun standards-time ((c Claims)) Bool
  (and (std-exp-numericdate c) (std-nbf-numericdate c) (std-iat-numericdate c)
       (std-exp-leeway expLeeway) (std-nbf-leeway nbfLeeway)
       (std-exp-reject c arbitraryClock expLeeway)
       (std-nbf-reject c arbitraryClock nbfLeeway)))
(push 1)
(assert (standards-time anyClaims))
(assert (not (std-exp-omitted anyClaims)))
(assert (not (std-nbf-omitted anyClaims)))
(assert (not (std-iat-omitted anyClaims)))
(check-sat)
(pop 1)
(push 1)
(assert (standards-time anyClaims))
(assert (std-exp-omitted anyClaims))
(assert (std-nbf-omitted anyClaims))
(assert (std-iat-omitted anyClaims))
(check-sat)
(pop 1)
