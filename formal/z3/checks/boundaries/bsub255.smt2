; BSUB255 sub of exactly 255 ASCII characters accepted
(declare-const context String)
(assert (= (str.len context) 237))
(assert (str.in_re context (re.* (str.to_re "x"))))
(assert (gates-current-example-aliases (store sample "sub" (Present (JString (str.++ "repo:org@1/repo@2:" context)))) 1000 "gates-oidc"))
(check-sat)
