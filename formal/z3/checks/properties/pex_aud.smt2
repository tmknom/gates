; PEX_AUD Exact-key candidate accepts alias AUD
(assert (gates-exact-name-candidate (store sample "AUD" (Present (JString "independent-value"))) 1000 "gates-oidc"))
(check-sat)
