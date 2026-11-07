; expect: sat sat sat unsat unsat unsat unsat unsat unsat unsat unsat sat unsat unsat
; EXAMPLES-SUB (jwt.md §2.2, §6.2 sub and its length): the model accepts only
; the immutable form, ASCII and at most 255 characters.
(push 1) (assert (accepts-with "sub" (Present (JString "repo:org@1/repo@2:environment:prod")))) (check-sat) (pop 1) ; immutable form: sat
(push 1) (assert (accepts-with "sub" (Present (JString "repo:org@1/repo@2:")))) (check-sat) (pop 1) ; empty context: sat
(push 1) (assert (accepts-with "sub" (Present (JString "repo:o:x@1/r/name@2:ctx")))) (check-sat) (pop 1) ; colon in owner name and slash in repository name: sat
(push 1) (assert (accepts-with "sub" (Present (JString "repo:org/repo:ctx")))) (check-sat) (pop 1) ; legacy form without IDs: unsat
(push 1) (assert (accepts-with "sub" (Present (JString "repo:org/repo@2:ctx")))) (check-sat) (pop 1) ; owner without @: unsat
(push 1) (assert (accepts-with "sub" (Present (JString "repo:o@x@1/repo@2:ctx")))) (check-sat) (pop 1) ; two @ in owner: unsat
(push 1) (assert (accepts-with "sub" (Present (JString "repo:@1/repo@2:ctx")))) (check-sat) (pop 1) ; empty owner name: unsat
(push 1) (assert (accepts-with "sub" (Present (JString "repo:org@1/repo@2")))) (check-sat) (pop 1) ; no colon after repository: unsat
(push 1) (assert (accepts-with "sub" (Present (JString "repository:org@1/repo@2:ctx")))) (check-sat) (pop 1) ; customized prefix: unsat
(push 1) (assert (accepts-with "sub" Absent)) (check-sat) (pop 1) ; missing: unsat
(push 1) (assert (accepts-with "sub" (Present (JString "repo:org@1/repo@2:\u{e9}")))) (check-sat) (pop 1) ; non-ASCII: unsat
(declare-const context255 String)
(declare-const context256 String)
(push 1) (assert (= (str.len context255) 237)) (assert (str.in_re context255 (re.* (str.to_re "x")))) (assert (accepts-with "sub" (Present (JString (str.++ "repo:org@1/repo@2:" context255))))) (check-sat) (pop 1) ; 255 characters: sat
(push 1) (assert (= (str.len context256) 238)) (assert (str.in_re context256 (re.* (str.to_re "x")))) (assert (accepts-with "sub" (Present (JString (str.++ "repo:org@1/repo@2:" context256))))) (check-sat) (pop 1) ; 256 characters: unsat
(push 1) (assert (accepts-with "sub" (Present (JInteger 1)))) (check-sat) (pop 1) ; JSON number: unsat
