; expect: unsat
; trace:checks GOJ-TIME-NBF GH-OIDC-TIME-EXAMPLE
; In the official example JWT, nbf is 600 seconds before iat, so GATES
; accepts its nbf at every now at which it accepts its iat.
(declare-const clock Int)
(assert (gates-time-iat gh-example-claims clock))
(assert (not (gates-time-nbf gh-example-claims clock)))
(check-sat)
