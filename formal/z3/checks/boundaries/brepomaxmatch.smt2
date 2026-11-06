; BREPOMAXMATCH repo max with corresponding sub value accepted
(assert (gates-current-example-aliases (store (store sample "repository_id" (Present (JString "9223372036854775807"))) "sub" (Present (JString "repo:org@1/repo@9223372036854775807:ctx"))) 1000 "gates-oidc"))
(check-sat)
