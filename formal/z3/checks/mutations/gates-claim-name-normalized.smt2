; expect: sat
; checks/safety/claim-name-match.smt2 finds a counterexample against
; bad-claim-name-normalized: two sets that agree under every name the rules
; read differ in acceptance through the fullwidth spelling of iss.
(declare-const anyValues Claims)
(define-fun without-iss () Claims (store sample "iss" Absent))
(define-fun otherClaims () Claims
  (lambda ((n String))
    (ite (read-name n) (select without-iss n) (select anyValues n))))
(assert (not (= (bad-claim-name-normalized without-iss 1000 "gates-oidc")
                (bad-claim-name-normalized otherClaims 1000 "gates-oidc"))))
(check-sat)
