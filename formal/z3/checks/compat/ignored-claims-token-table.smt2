; expect: unsat unsat
; trace:checks GOJ-REPOSITORY GOJ-REPOSITORY-OWNER GOJ-RUN-NUMBER GOJ-WORKFLOW GOJ-HEAD-REF GOJ-BASE-REF GOJ-REF-TYPE GOJ-REPOSITORY-VISIBILITY GOJ-RUNNER-ENVIRONMENT GOJ-CHECK-RUN-ID GH-OIDC-CLAIMS-TOKEN-TABLE
; The claims table describes these Claims, which GATES does not use.
; The first check-sat shows GATES consults none of the names; the second
; that a Claim under any of them, of any value or absent, leaves acceptance as
; it is.
(declare-const name String)
(declare-const member Member)
(define-fun described ((n String)) Bool
  (and (gh-claims-token-table-name n)
       (or (= n "repository") (= n "repository_owner") (= n "run_number")
           (= n "workflow") (= n "head_ref") (= n "base_ref") (= n "ref_type")
           (= n "repository_visibility") (= n "runner_environment")
           (= n "check_run_id"))))
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
