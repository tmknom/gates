; BSUB256 sub of 256 ASCII characters rejected
(declare-const context String)
(assert (= (str.len context) 238))
(assert (str.in_re context (re.* (str.to_re "x"))))
(assert (gates-current-example-aliases (store sample "sub" (Present (JString (str.++ "repo:org@1/repo@2:" context)))) 1000 "gates-oidc"))
(check-sat)
