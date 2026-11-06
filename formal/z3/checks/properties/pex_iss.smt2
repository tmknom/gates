; PEX_ISS Exact-key candidate accepts alias ISS
(assert (gates-exact-name-candidate (store sample "ISS" (Present (JString "independent-value"))) 1000 "gates-oidc"))
(check-sat)
