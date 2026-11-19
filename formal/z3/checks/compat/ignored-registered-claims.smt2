; expect: unsat unsat
; trace:checks GOJ-JTI GH-OIDC-REGISTERED-CLAIMS
; GH-OIDC-REGISTERED-CLAIMS gives jti the meaning of a unique identifier of
; the token, which GATES does not use.
; The first check-sat shows GATES consults none of the names; the second
; that a Claim under any of them, of any value or absent, leaves acceptance as
; it is.
(declare-const name String)
(declare-const member Member)
(define-fun described ((n String)) Bool
  (= n "jti"))
(push 1)
(assert (described name))
(assert (consulted-name name))
(check-sat)
(pop 1)
(push 1)
(assert (described name))
(assert (claim-changes-acceptance name member))
(check-sat)
(pop 1)
