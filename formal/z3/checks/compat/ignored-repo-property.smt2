; expect: unsat unsat
; trace:checks GOJ-DYNAMIC-CLAIM GH-OIDC-REPO-PROPERTY-PRESENCE GH-OIDC-REPO-PROPERTY-VALUE
; A repo_property_* Claim is present when an administrator adds the property
; and the repository has a value, and its value is a string. GATES does not
; use any of them.
; The first check-sat shows GATES consults none of the names; the second
; that a Claim under any of them, of any value or absent, leaves acceptance as
; it is.
(declare-const name String)
(declare-const member Member)
(define-fun described ((n String)) Bool
  (str.prefixof "repo_property_" n))
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
