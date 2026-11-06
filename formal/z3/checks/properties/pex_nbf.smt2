; PEX_NBF Exact-key candidate accepts alias NBF
(assert (gates-exact-name-candidate (store sample "NBF" (Present (JString "independent-value"))) 1000 "gates-oidc"))
(check-sat)
