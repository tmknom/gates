; expect: unsat
; trace:checks GOJ-HEADER-ALG RFC7515-ALG-REQUIRED RFC7515-ALG-UNDERSTAND RFC7515-ALG-APPLICATION-ACCEPTABLE RFC7515-STRING-COMPARE RFC7519-ALG-ACCEPTABLE RFC8725-ALG-SET-SPECIFY RFC8725-ALG-SET-RESTRICT RFC8725-ALG-CURRENT
; GATES implements, accepts, specifies to the library and considers current
; the one algorithm RS256. Every JOSE Header GATES accepts has alg, and its alg
; is in that set and equal to RS256 under the comparison of RFC 7515.
(declare-const anyHeader Header)
(assert (gates-header-accepted anyHeader))
(define-fun rs256-only () (Array String Bool)
  (store ((as const (Array String Bool)) false) "RS256" true))
(assert (not (and (std-jws-alg-present anyHeader)
                  (std-jws-alg-understood anyHeader rs256-only)
                  (std-jws-alg-application-acceptable anyHeader rs256-only)
                  (std-jwt-alg-acceptable anyHeader rs256-only)
                  (std-alg-set-restrict anyHeader rs256-only)
                  (std-alg-current anyHeader rs256-only)
                  (string-member anyHeader "alg")
                  (std-jws-string-equal (string-member-value anyHeader "alg") "RS256"))))
(check-sat)
