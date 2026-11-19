; expect: unsat unsat
; trace:checks GOJ-WORKFLOW-REF GOJ-JOB-WORKFLOW-REF GOJ-JOB-WORKFLOW-SHA GH-OIDC-WORKFLOW-REF-OBSERVED
; GH-OIDC-WORKFLOW-REF-OBSERVED records when job_workflow_ref equals
; workflow_ref and what job_workflow_sha holds; GATES reads none of them, so
; it accepts every such issuance without relying on the records.
; The first check-sat shows GATES consults none of the names; the second
; that a Claim under any of them, of any value or absent, leaves acceptance as
; it is.
(declare-const name String)
(declare-const member Member)
(define-fun described ((n String)) Bool
  (or (= n "workflow_ref") (= n "job_workflow_ref") (= n "job_workflow_sha")))
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
