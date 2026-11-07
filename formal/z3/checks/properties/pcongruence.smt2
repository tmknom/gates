; expect: unsat
; PCONGRUENCE: two Claims Sets with equal values under every consulted name
; have equal acceptance, so consulted-name covers every name the acceptance
; predicate reads. otherClaims ranges over every set that agrees with
; anyClaims on the consulted names, because anyValues is arbitrary.
(declare-const anyClaims Claims)
(declare-const anyValues Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(define-fun otherClaims () Claims
  (lambda ((n String))
    (ite (consulted-name n) (select anyClaims n) (select anyValues n))))
(assert (not (= (gates-current anyClaims arbitraryClock arbitraryAudience)
                (gates-current otherClaims arbitraryClock arbitraryAudience))))
(check-sat)
