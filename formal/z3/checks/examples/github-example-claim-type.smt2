; expect: unsat sat
; trace:checks GOJ-EXP GOJ-IAT GOJ-NBF GOJ-ID-FORMAT GH-OIDC-CLAIM-TYPE-EXAMPLE
; The official example JWT has exp, iat and nbf that GOJ-EXP, GOJ-IAT and
; GOJ-NBF accept, and repository_id, repository_owner_id and run_attempt that
; GOJ-ID-FORMAT accepts. Its run_id is a placeholder that shows only the type.
; The second check puts these values into sample, with the immutable sub of
; the same IDs, and GATES accepts the result at one second after iat.
(define-fun example-member ((n String)) Member (select gh-example-claims n))
(push 1)
(assert (not (and (gates-exp gh-example-claims) (gates-iat gh-example-claims)
                  (gates-nbf gh-example-claims)
                  (gates-id-claim gh-example-claims "repository_id")
                  (gates-id-claim gh-example-claims "repository_owner_id")
                  (gates-id-claim gh-example-claims "run_attempt"))))
(check-sat)
(pop 1)
(push 1)
(assert (gates-claims-accepted
          (store (store (store (store (store (store (store sample
            "exp" (example-member "exp")) "iat" (example-member "iat"))
            "nbf" (example-member "nbf"))
            "repository_id" (example-member "repository_id"))
            "repository_owner_id" (example-member "repository_owner_id"))
            "run_attempt" (example-member "run_attempt"))
            "sub" (Present (JString "repo:octo-org@65/octo-repo@74:environment:prod")))
          1632493568 "gates-oidc"))
(check-sat)
(pop 1)
