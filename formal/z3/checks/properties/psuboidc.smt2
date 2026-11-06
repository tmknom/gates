; PSUBOIDC (universal): every GATES-accepted sub satisfies OIDC Core §2 ASCII/255
(declare-const anyClaims Claims)
(assert (gates-current-example-aliases anyClaims 1000 "gates-oidc"))
(assert (not (oidc-sub-conditional anyClaims)))
(check-sat)
