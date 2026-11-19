; expect: sat
; trace:checks GOJ-TIME-NBF RFC7519-NBF-LEEWAY
; checks/safety/time-nbf-reject.smt2 finds a counterexample with a leeway of
; 299 seconds, so GATES relies on the leeway that RFC7519-NBF-LEEWAY allows and
; the check is not vacuous. The fixed sub and ID values only guide the search.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(assert (= (select anyClaims "sub") (Present (JString "repo:o@1/r@2:c"))))
(assert (= (select anyClaims "repository_owner_id") (Present (JString "1"))))
(assert (= (select anyClaims "repository_id") (Present (JString "2"))))
(assert (gates-claims-accepted anyClaims arbitraryClock arbitraryAudience))
(assert (not (std-nbf-reject anyClaims arbitraryClock 299.0)))
(check-sat)
