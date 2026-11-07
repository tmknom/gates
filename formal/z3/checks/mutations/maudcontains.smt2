; expect: sat
; MAUDCONTAINS: PAUDUNIQUE finds a counterexample against bad-aud-contains.
; The fixed aud, sub and ID values only guide the search for the counterexample.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const audience String)
(declare-const otherAudience String)
(assert (= (select anyClaims "aud")
           (Present (JStringArray (seq.++ (seq.unit "a") (seq.unit "b"))))))
(assert (= (select anyClaims "sub") (Present (JString "repo:o@1/r@2:c"))))
(assert (= (select anyClaims "repository_owner_id") (Present (JString "1"))))
(assert (= (select anyClaims "repository_id") (Present (JString "2"))))
(assert (bad-aud-contains anyClaims arbitraryClock audience))
(assert (bad-aud-contains anyClaims arbitraryClock otherAudience))
(assert (not (= audience otherAudience)))
(check-sat)
