; expect: sat
; checks/safety/claim-name-match.smt2 finds a counterexample against
; bad-reject-extra. The fixed sub and ID values only guide the search.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(declare-const anyValues Claims)
(assert (= (select anyClaims "sub") (Present (JString "repo:o@1/r@2:c"))))
(assert (= (select anyClaims "repository_owner_id") (Present (JString "1"))))
(assert (= (select anyClaims "repository_id") (Present (JString "2"))))
(define-fun otherClaims () Claims
  (lambda ((n String))
    (ite (consulted-name n) (select anyClaims n) (select anyValues n))))
(assert (not (= (bad-reject-extra anyClaims arbitraryClock arbitraryAudience)
                (bad-reject-extra otherClaims arbitraryClock arbitraryAudience))))
(check-sat)
