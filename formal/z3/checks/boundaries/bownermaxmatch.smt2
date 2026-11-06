; BOWNERMAXMATCH owner max with corresponding sub value accepted
(assert (gates-current-example-aliases (store (store sample "repository_owner_id" (Present (JString "9223372036854775807"))) "sub" (Present (JString "repo:org@9223372036854775807/repo@2:ctx"))) 1000 "gates-oidc"))
(check-sat)
