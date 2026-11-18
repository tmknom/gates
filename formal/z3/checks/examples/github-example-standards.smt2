; expect: sat sat
; trace:checks GH-OIDC-CLAIM-TYPE-EXAMPLE GH-OIDC-TIME-EXAMPLE GH-OIDC-SUB-EXAMPLE-LEGACY GH-OIDC-HEADER-EXAMPLE RFC7519-ISS-FORMAT RFC7519-SUB-FORMAT RFC7519-JTI-FORMAT RFC7519-EXP-NUMERICDATE RFC7519-IAT-NUMERICDATE RFC7519-NBF-NUMERICDATE RFC7519-AUD-SINGLE-STRING RFC7519-TYP-VALUE RFC7515-ALG-REQUIRED RFC7515-ALG-FORMAT RFC7515-KID-FORMAT RFC7515-KEY-IDENTIFY RFC7518-NONE-ACCEPT-DEFAULT RFC7797-JWT-B64-FALSE
; The official example JWT satisfies the clauses of the standards on what
; the Issuer puts in the Claims Set and the Header. A value that did not would
; show a misreading of a clause or an example against the standard.
(push 1)
(assert (and (std-iss-format gh-example-claims) (std-sub-format gh-example-claims)
             (std-jti-format gh-example-claims) (std-exp-numericdate gh-example-claims)
             (std-iat-numericdate gh-example-claims) (std-nbf-numericdate gh-example-claims)
             (std-aud-single-string (member-value (select gh-example-claims "aud")) true)))
(check-sat)
(pop 1)
(push 1)
(assert (and (std-jwt-typ-value gh-example-header) (std-jws-alg-present gh-example-header)
             (std-jws-alg-format gh-example-header) (std-jws-kid-format gh-example-header)
             (std-jws-key-identify gh-example-header false)
             (std-none-accept-default gh-example-header false)
             (std-jwt-b64-not-false gh-example-header)))
(check-sat)
(pop 1)
