; expect: unsat
; PTIME (jwt.md §2.4, §2.38.3): in an accepted set, exp and iat are JSON
; integers within 0..253402300799, and nbf is absent or such an integer.
(define-fun time-integer ((m MaybeClaim)) Bool
  (exists ((t Int))
    (and (= m (Present (JInteger t)))
         (<= 0 t)
         (<= t 253402300799))))
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(assert (gates-current anyClaims arbitraryClock arbitraryAudience))
(assert (not (and (time-integer (select anyClaims "exp"))
                  (time-integer (select anyClaims "iat"))
                  (or (= (select anyClaims "nbf") Absent)
                      (time-integer (select anyClaims "nbf"))))))
(check-sat)
