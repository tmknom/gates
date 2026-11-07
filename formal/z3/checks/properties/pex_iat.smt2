; PEX_IAT Exact-key candidate accepts alias IAT
(assert (gates-exact-name-candidate (store sample "IAT" (Present (JString "independent-value"))) 1000 "gates-oidc"))
(check-sat)
