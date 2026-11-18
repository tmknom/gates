; expect: unsat unsat unsat unsat unsat unsat
; trace:checks GOJ-SUB-FORMAT GH-OIDC-SUB-OBSERVED
; GATES accepts the sub of every issuance the Observed clauses allow as well,
; where sub has the immutable form whatever the repository state, with each
; recorded context. The names, IDs and contexts are fixed to the recorded ones,
; in the two steps of sub-format-documented.smt2.
(define-fun issued-with-context ((value String)) Bool
  (and observed-issuance observed-names (= context value)))
(define-fun sub-is ((s String)) Bool
  (= (select issued "sub") (Present (JString s))))
(define-fun accepts-sub ((s String)) Bool
  (gates-sub-format (store issued "sub" (Present (JString s)))))
(push 1) (assert (issued-with-context "ref:refs/heads/test-branch")) (assert (not (sub-is "repo:octocat@583231/example-repo@1234567890:ref:refs/heads/test-branch"))) (check-sat) (pop 1)
(push 1) (assert (not (accepts-sub "repo:octocat@583231/example-repo@1234567890:ref:refs/heads/test-branch"))) (check-sat) (pop 1)
(push 1) (assert (issued-with-context "pull_request")) (assert (not (sub-is "repo:octocat@583231/example-repo@1234567890:pull_request"))) (check-sat) (pop 1)
(push 1) (assert (not (accepts-sub "repo:octocat@583231/example-repo@1234567890:pull_request"))) (check-sat) (pop 1)
(push 1) (assert (issued-with-context "environment:testing")) (assert (not (sub-is "repo:octocat@583231/example-repo@1234567890:environment:testing"))) (check-sat) (pop 1)
(push 1) (assert (not (accepts-sub "repo:octocat@583231/example-repo@1234567890:environment:testing"))) (check-sat) (pop 1)
