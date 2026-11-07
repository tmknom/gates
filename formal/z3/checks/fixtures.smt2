; Compact accepted decoded Claims Set, with all other names absent.
; now=1000, expected audience="gates-oidc". NOT a real GitHub-issued JWT.
(define-fun sample () Claims
  (store (store (store (store (store (store (store (store (store
    ((as const Claims) Absent)
    "iss" (Present (JString "https://token.actions.githubusercontent.com")))
    "aud" (Present (JString "gates-oidc")))
    "exp" (Present (JInteger 1001)))
    "iat" (Present (JInteger 1000)))
    "sub" (Present (JString "repo:org@1/repo@2:ctx")))
    "repository_owner_id" (Present (JString "1")))
    "repository_id" (Present (JString "2")))
    "run_id" (Present (JString "3")))
    "run_attempt" (Present (JString "1"))))
; Acceptance of sample with one Claim replaced, at now=1000 for "gates-oidc".
(define-fun accepts-with ((name String) (value MaybeClaim)) Bool
  (gates-current (store sample name value) 1000 "gates-oidc"))
