; MEXP Bad inclusive expiration: mutant accepts and normal rejects
(assert (and (bad-exp-inclusive (store sample "exp" (Present (JInteger 1000))) 1000 "gates-oidc") (not (gates-current-example-aliases (store sample "exp" (Present (JInteger 1000))) 1000 "gates-oidc"))))
(check-sat)
