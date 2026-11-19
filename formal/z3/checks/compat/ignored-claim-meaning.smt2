; expect: unsat unsat
; trace:checks GOJ-SHA GOJ-ENTERPRISE GOJ-ENTERPRISE-ID GOJ-REF-PROTECTED GOJ-ENVIRONMENT-NODE-ID GOJ-ISSUER-SCOPE GH-OIDC-CLAIM-MEANING
; GitHub does not give the meaning of these Claims, which GATES only records
; or does not use, so whatever GitHub issues under them is accepted.
; The first check-sat shows GATES consults none of the names; the second
; that a Claim under any of them, of any value or absent, leaves acceptance as
; it is.
(declare-const name String)
(declare-const member Member)
(define-fun described ((n String)) Bool
  (or (= n "sha") (= n "enterprise") (= n "enterprise_id") (= n "ref_protected")
      (= n "environment_node_id") (= n "issuer_scope")))
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
