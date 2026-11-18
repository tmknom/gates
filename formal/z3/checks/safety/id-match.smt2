; expect: unsat
; trace:checks GOJ-ID-MATCH RFC7519-STRING-COMPARE
; In every Claims Set GATES accepts, the two IDs in sub equal
; repository_owner_id and repository_id under the comparison of the clause.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(assert (gates-claims-accepted anyClaims arbitraryClock arbitraryAudience))
(assert (not (and (std-jwt-string-equal (string-member-value anyClaims "repository_owner_id") (sub-owner-id anyClaims))
                  (std-jwt-string-equal (string-member-value anyClaims "repository_id") (sub-repository-id anyClaims)))))
(check-sat)
