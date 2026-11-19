; expect: unsat
; trace:checks GOJ-JTI GOJ-REPOSITORY GOJ-REPOSITORY-OWNER GOJ-ENTERPRISE GOJ-ENTERPRISE-ID GOJ-RUN-NUMBER GOJ-WORKFLOW GOJ-WORKFLOW-SHA GOJ-HEAD-REF GOJ-BASE-REF GOJ-REF-TYPE GOJ-REF-PROTECTED GOJ-ENVIRONMENT-NODE-ID GOJ-JOB-WORKFLOW-SHA GOJ-REPOSITORY-VISIBILITY GOJ-RUNNER-ENVIRONMENT GOJ-ISSUER-SCOPE GOJ-CHECK-RUN-ID GOJ-REF GOJ-SHA GOJ-ACTOR GOJ-ACTOR-ID GOJ-WORKFLOW-REF GOJ-EVENT-NAME GOJ-ENVIRONMENT GOJ-JOB-WORKFLOW-REF
; The acceptance rules consult none of the unused and audit Claims, so with
; checks/safety/claim-unknown-ignore.smt2 a Claim under any of them, of any
; value or absent, leaves acceptance as it is.
(declare-const name String)
(assert (or (gates-unused-name name) (gates-audit-name name)))
(assert (consulted-name name))
(check-sat)
