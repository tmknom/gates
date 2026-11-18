; expect: sat
; MEXP: the safety check of GOJ-TIME against RFC7519-EXP-REJECT finds a
; counterexample against bad-exp-inclusive.
; The fixed sub and ID values only guide the search for the counterexample.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(assert (= (select anyClaims "sub") (Present (JString "repo:o@1/r@2:c"))))
(assert (= (select anyClaims "repository_owner_id") (Present (JString "1"))))
(assert (= (select anyClaims "repository_id") (Present (JString "2"))))
(assert (bad-exp-inclusive anyClaims arbitraryClock arbitraryAudience))
(assert (not (std-exp-reject anyClaims arbitraryClock 0.0)))
(check-sat)
