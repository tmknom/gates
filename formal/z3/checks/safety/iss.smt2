; expect: unsat
; trace:checks GOJ-ISS RFC7519-ISS-FORMAT RFC7519-STRING-COMPARE RFC7519-STRINGORURI-COMPARE
; Every Claims Set GATES accepts has an iss that is a string, equal to the
; fixed Issuer under the comparison of both clauses.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(assert (gates-claims-accepted anyClaims arbitraryClock arbitraryAudience))
(assert (not (and (std-iss-format anyClaims)
                  (string-member anyClaims "iss")
                  (std-jwt-string-equal (string-member-value anyClaims "iss") gates-fixed-issuer)
                  (std-stringoruri-equal (string-member-value anyClaims "iss") gates-fixed-issuer))))
(check-sat)
