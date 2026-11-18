; expect: unsat unsat unsat unsat unsat unsat
; trace:checks GOJ-SUB-FORMAT GH-OIDC-SUB-AT-SEPARATOR GH-OIDC-SUB-CONTEXT GH-OIDC-SUB-IMMUTABLE-DEFAULT GH-OIDC-SUB-IMMUTABLE-FORMAT
; GATES accepts the sub of every issuance the Documented clauses allow for a
; repository whose default sub has the immutable form, once sub is present
; (GH-OIDC-CLAIM-PRESENCE), with a context of each of the three kinds. The
; names and IDs are fixed to those of the clause (documented-names): names
; that keep the parts apart (GH-OIDC-SUB-LENGTH) and IDs with the GOJ-ID-FORMAT
; spelling (GH-OIDC-CLAIM-ID-FORMAT). Z3 answers unknown on str.indexof over a
; sub it has to solve for, so each context takes two steps: the clauses fix
; sub to one string, and GATES accepts that string. gates-sub-format reads sub
; alone, so the two steps show the acceptance.
(define-fun issued-with-context ((value String)) Bool
  (and documented-issuance immutable-repository documented-names
       (= context value) ((_ is Present) (select issued "sub"))))
(define-fun sub-is ((s String)) Bool
  (= (select issued "sub") (Present (JString s))))
(define-fun accepts-sub ((s String)) Bool
  (gates-sub-format (store issued "sub" (Present (JString s)))))
(push 1) (assert (issued-with-context "ref:refs/heads/main")) (assert (not (sub-is "repo:octo-org@123456/octo-repo@456789:ref:refs/heads/main"))) (check-sat) (pop 1)
(push 1) (assert (not (accepts-sub "repo:octo-org@123456/octo-repo@456789:ref:refs/heads/main"))) (check-sat) (pop 1)
(push 1) (assert (issued-with-context "pull_request")) (assert (not (sub-is "repo:octo-org@123456/octo-repo@456789:pull_request"))) (check-sat) (pop 1)
(push 1) (assert (not (accepts-sub "repo:octo-org@123456/octo-repo@456789:pull_request"))) (check-sat) (pop 1)
(push 1) (assert (issued-with-context "environment:prod")) (assert (not (sub-is "repo:octo-org@123456/octo-repo@456789:environment:prod"))) (check-sat) (pop 1)
(push 1) (assert (not (accepts-sub "repo:octo-org@123456/octo-repo@456789:environment:prod"))) (check-sat) (pop 1)
