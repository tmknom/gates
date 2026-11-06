; PEX_EXP Exact-key candidate accepts alias EXP
(assert (gates-exact-name-candidate (store sample "EXP" (Present (JString "independent-value"))) 1000 "gates-oidc"))
(check-sat)
