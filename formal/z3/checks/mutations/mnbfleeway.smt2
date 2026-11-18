; expect: sat
; trace:checks GOJ-TIME RFC7519-NBF-LEEWAY
; MNBFLEEWAY: the safety check of GOJ-TIME against RFC7519-NBF-REJECT finds a
; counterexample with a leeway of 299 seconds, so GATES relies on the leeway
; that RFC7519-NBF-LEEWAY allows and the check is not vacuous.
; The fixed sub and ID values only guide the search for the counterexample.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(assert (= (select anyClaims "sub") (Present (JString "repo:o@1/r@2:c"))))
(assert (= (select anyClaims "repository_owner_id") (Present (JString "1"))))
(assert (= (select anyClaims "repository_id") (Present (JString "2"))))
(assert (gates-current anyClaims arbitraryClock arbitraryAudience))
(assert (not (std-nbf-reject anyClaims arbitraryClock 299.0)))
(check-sat)
