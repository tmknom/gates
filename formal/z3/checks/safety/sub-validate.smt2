; expect: unsat
; trace:checks GOJ-SUB-FORMAT RFC8725-SUB-VALIDATE
; The subjects valid at GATES, for the fixed Issuer, are the sub values in the
; immutable form whose two IDs match the ID Claims of the same Claims Set.
; Every Claims Set GATES accepts has a sub that is such a valid subject.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(assert (gates-claims-accepted anyClaims arbitraryClock arbitraryAudience))
(define-fun valid-subject () (Array String Bool)
  (lambda ((s String))
    (let ((d (store anyClaims "sub" (Present (JString s)))))
      (and (gates-sub-format d) (gates-id-match d)))))
(assert (not (std-sub-validate anyClaims valid-subject)))
(check-sat)
