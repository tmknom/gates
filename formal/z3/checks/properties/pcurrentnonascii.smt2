; PCURRENTNONASCII Current rejects NONASCII subject
(assert (gates-current-example-aliases (store sample "sub" (Present (JString "repo:org@1/repo@2:é"))) 1000 "gates-oidc"))
(check-sat)
