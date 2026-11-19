; expect: sat
; checks/safety/iss.smt2 finds a counterexample against
; bad-iss-case-insensitive.
(declare-const anyClaims Claims)
(assert (bad-iss-case-insensitive anyClaims))
(assert (not (and (string-member anyClaims "iss")
                  (std-jwt-string-equal (string-member-value anyClaims "iss") gates-fixed-issuer))))
(check-sat)
