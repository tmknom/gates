; expect: unsat
; trace:checks GOJ-EXP GOJ-IAT GOJ-NBF
; In an accepted set, exp and iat are JSON integers within 0..253402300799,
; and nbf is absent or such an integer.
(define-fun time-integer ((m Member)) Bool
  (exists ((t Int))
    (and (= m (Present (JInteger t)))
         (<= 0 t)
         (<= t 253402300799))))
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(assert (gates-claims-accepted anyClaims arbitraryClock arbitraryAudience))
(assert (not (and (time-integer (select anyClaims "exp"))
                  (time-integer (select anyClaims "iat"))
                  (or (= (select anyClaims "nbf") Absent)
                      (time-integer (select anyClaims "nbf"))))))
(check-sat)
