; expect: unsat unsat unsat unsat unsat unsat sat unsat unsat sat unsat unsat unsat sat unsat unsat unsat sat unsat unsat
; EXAMPLES-ID (jwt.md §2.38.1, §2.38.2, §2.17, §2.19, §6.2 不変 ID and
; run_attempt): the model accepts only positive canonical int64 strings, and
; only owner and repository IDs that match sub.
(push 1) (assert (accepts-with "repository_owner_id" (Present (JString "0")))) (check-sat) (pop 1) ; owner zero: unsat
(push 1) (assert (accepts-with "repository_owner_id" (Present (JString "01")))) (check-sat) (pop 1) ; owner leading zero: unsat
(push 1) (assert (accepts-with "repository_owner_id" (Present (JInteger 1)))) (check-sat) (pop 1) ; owner JSON number: unsat
(push 1) (assert (accepts-with "repository_owner_id" (Present (JString "+1")))) (check-sat) (pop 1) ; owner sign: unsat
(push 1) (assert (accepts-with "repository_owner_id" (Present (JString "9")))) (check-sat) (pop 1) ; owner not matching sub: unsat
(push 1) (assert (gates-current (store (store sample "repository_owner_id" (Present (JString "9223372036854775808"))) "sub" (Present (JString "repo:org@9223372036854775808/repo@2:ctx"))) 1000 "gates-oidc")) (check-sat) (pop 1) ; owner above int64 with matching sub: unsat
(push 1) (assert (gates-current (store (store sample "repository_owner_id" (Present (JString "9223372036854775807"))) "sub" (Present (JString "repo:org@9223372036854775807/repo@2:ctx"))) 1000 "gates-oidc")) (check-sat) (pop 1) ; owner max with matching sub: sat
(push 1) (assert (accepts-with "repository_id" (Present (JString "0")))) (check-sat) (pop 1) ; repository zero: unsat
(push 1) (assert (accepts-with "repository_id" (Present (JString "9")))) (check-sat) (pop 1) ; repository not matching sub: unsat
(push 1) (assert (gates-current (store (store sample "repository_id" (Present (JString "9223372036854775807"))) "sub" (Present (JString "repo:org@1/repo@9223372036854775807:ctx"))) 1000 "gates-oidc")) (check-sat) (pop 1) ; repository max with matching sub: sat
(push 1) (assert (accepts-with "run_id" (Present (JString "0")))) (check-sat) (pop 1) ; run_id zero: unsat
(push 1) (assert (accepts-with "run_id" (Present (JString "03")))) (check-sat) (pop 1) ; run_id leading zero: unsat
(push 1) (assert (accepts-with "run_id" (Present (JString "9223372036854775808")))) (check-sat) (pop 1) ; run_id above int64: unsat
(push 1) (assert (accepts-with "run_id" (Present (JString "9223372036854775807")))) (check-sat) (pop 1) ; run_id max: sat
(push 1) (assert (accepts-with "run_id" Absent)) (check-sat) (pop 1) ; run_id missing: unsat
(push 1) (assert (accepts-with "run_attempt" (Present (JString "0")))) (check-sat) (pop 1) ; run_attempt zero: unsat
(push 1) (assert (accepts-with "run_attempt" (Present (JString "01")))) (check-sat) (pop 1) ; run_attempt leading zero: unsat
(push 1) (assert (accepts-with "run_attempt" (Present (JString "2")))) (check-sat) (pop 1) ; run_attempt re-run: sat
(push 1) (assert (accepts-with "run_attempt" Absent)) (check-sat) (pop 1) ; run_attempt missing: unsat
(push 1) (assert (accepts-with "run_attempt" (Present (JInteger 1)))) (check-sat) (pop 1) ; run_attempt JSON number: unsat
