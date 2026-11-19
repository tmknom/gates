; expect: unsat unsat
; trace:checks GOJ-EVENT-NAME GH-OIDC-EVENT-NAME GH-OIDC-EVENT-NAME-DEPENDABOT
; GH-OIDC-EVENT-NAME gives the meaning of event_name, and
; GH-OIDC-EVENT-NAME-DEPENDABOT its value dynamic in a Dependabot update job.
; GATES only records it.
; The first check-sat shows GATES consults none of the names; the second
; that a Claim under any of them, of any value or absent, leaves acceptance as
; it is.
(declare-const name String)
(declare-const member Member)
(define-fun described ((n String)) Bool
  (= n "event_name"))
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
