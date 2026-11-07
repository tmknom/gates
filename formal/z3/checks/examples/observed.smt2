; expect: sat sat sat sat sat sat
; EXAMPLES-OBSERVED (docs/research/github/oidc-jwt.md): the ten consulted
; Claims of the six JWTs that GitHub actually issued are accepted one second
; after iat. They were fetched without an Audience, so the default Audience is
; given as the expected one. Observations do not show that GitHub always issues
; this form.
(define-fun observed ((exp Int) (iat Int) (nbf Int) (sub String) (run String)) Claims
  (store (store (store (store (store (store (store (store (store (store
    ((as const Claims) Absent)
    "iss" (Present (JString "https://token.actions.githubusercontent.com")))
    "aud" (Present (JString "https://github.com/octocat")))
    "exp" (Present (JInteger exp)))
    "iat" (Present (JInteger iat)))
    "nbf" (Present (JInteger nbf)))
    "sub" (Present (JString sub)))
    "repository_owner_id" (Present (JString "583231")))
    "repository_id" (Present (JString "1234567890")))
    "run_id" (Present (JString run)))
    "run_attempt" (Present (JString "1"))))
(push 1) (assert (gates-current (observed 1793340472 1793340172 1793339872 "repo:octocat@583231/example-repo@1234567890:ref:refs/heads/test-branch" "41011783574") 1793340173 "https://github.com/octocat")) (check-sat) (pop 1) ; push: sat
(push 1) (assert (gates-current (observed 1793340937 1793340637 1793340337 "repo:octocat@583231/example-repo@1234567890:pull_request" "41012535428") 1793340638 "https://github.com/octocat")) (check-sat) (pop 1) ; pull_request: sat
(push 1) (assert (gates-current (observed 1793422055 1793421755 1793421455 "repo:octocat@583231/example-repo@1234567890:ref:refs/heads/main" "41169659213") 1793421756 "https://github.com/octocat")) (check-sat) (pop 1) ; workflow_dispatch: sat
(push 1) (assert (gates-current (observed 1793344300 1793344000 1793343700 "repo:octocat@583231/example-repo@1234567890:ref:refs/heads/test-branch" "41018050776") 1793344001 "https://github.com/octocat")) (check-sat) (pop 1) ; absolute reusable workflow: sat
(push 1) (assert (gates-current (observed 1793343920 1793343620 1793343320 "repo:octocat@583231/example-repo@1234567890:pull_request" "41017362820") 1793343621 "https://github.com/octocat")) (check-sat) (pop 1) ; relative reusable workflow: sat
(push 1) (assert (gates-current (observed 1793343910 1793343610 1793343310 "repo:octocat@583231/example-repo@1234567890:environment:testing" "41017362820") 1793343611 "https://github.com/octocat")) (check-sat) (pop 1) ; environment: sat
