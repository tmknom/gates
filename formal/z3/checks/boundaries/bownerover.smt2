; BOWNEROVER repository_owner_id greater than int64 max rejected
(assert (gates-current-example-aliases (store sample "repository_owner_id" (Present (JString "9223372036854775808"))) 1000 "gates-oidc"))
(check-sat)
