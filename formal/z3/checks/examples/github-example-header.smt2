; expect: unsat
; trace:checks GOJ-HEADER-ALG GOJ-HEADER-KID GOJ-HEADER-TYP GOJ-HEADER-KEY-REFERENCE GH-OIDC-HEADER-EXAMPLE
; GATES accepts the Header of the official example JWT: alg RS256, a kid, typ
; and x5t, of which it reads alg and kid.
(assert (not (and (gates-header-accepted gh-example-header)
                  (gates-header-typ-name "typ")
                  (not (header-consulted-name "typ"))
                  (not (header-consulted-name "x5t")))))
(check-sat)
