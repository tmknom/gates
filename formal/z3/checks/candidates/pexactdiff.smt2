; expect: sat
; PEXACTDIFF: a set exists that the exact-name candidate accepts and the
; current design rejects. The fixed sub and ID values only guide the search.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(assert (= (select anyClaims "sub") (Present (JString "repo:o@1/r@2:c"))))
(assert (= (select anyClaims "repository_owner_id") (Present (JString "1"))))
(assert (= (select anyClaims "repository_id") (Present (JString "2"))))
(assert (gates-exact-name-candidate anyClaims arbitraryClock arbitraryAudience))
(assert (not (gates-current anyClaims arbitraryClock arbitraryAudience)))
(check-sat)
