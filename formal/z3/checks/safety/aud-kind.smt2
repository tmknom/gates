; expect: unsat
; trace:checks GOJ-AUD RFC8725-KIND-CONFUSION-MITIGATE RFC8725-KIND-EXCLUSIVE
; The same Issuer issues the AWS OIDC JWT, which AWS STS validates by finding
; sts.amazonaws.com in aud. For a GATES Audience other than that value
; (INV-005), no Claims Set is accepted by GATES and by that validation, so a
; JWT of the other kind is never accepted as a GATES OIDC JWT.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(assert (gates-claims-accepted anyClaims arbitraryClock arbitraryAudience))
(assert (not (= arbitraryAudience "sts.amazonaws.com")))
(define-fun aws-kind () Bool (std-aud-identifies anyClaims "sts.amazonaws.com"))
(assert (not (and (std-kind-confusion-mitigate true aws-kind true)
                  (std-kind-exclusive true true aws-kind))))
(check-sat)
