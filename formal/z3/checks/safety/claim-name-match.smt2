; expect: unsat
; trace:checks GOJ-CLAIM-NAME-MATCH OIDCC-STRING-CODE-POINT OIDCC-STRING-NO-NORMALIZATION RFC7519-STRING-COMPARE
; A Claim is identified by a name equal, under the comparison of the clauses,
; to one of the names the rules read (read-name in checks/fixtures.smt2); the
; aliases are consulted too, in order to reject them. Two Claims Sets that
; agree under every such name are accepted alike, so a value under any other
; name, such as one in another normalization form, is not read as a consulted
; Claim. otherClaims ranges over every set that agrees with anyClaims under
; those names, because anyValues is arbitrary.
(declare-const anyClaims Claims)
(declare-const anyValues Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(define-fun otherClaims () Claims
  (lambda ((n String))
    (ite (read-name n) (select anyClaims n) (select anyValues n))))
(assert (not (= (gates-claims-accepted anyClaims arbitraryClock arbitraryAudience)
                (gates-claims-accepted otherClaims arbitraryClock arbitraryAudience))))
(check-sat)
