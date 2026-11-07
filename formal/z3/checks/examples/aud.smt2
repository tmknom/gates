; expect: sat sat unsat unsat unsat unsat unsat unsat
; EXAMPLES-AUD (jwt.md §2.3, §6.2 aud): the model accepts only the GATES
; Audience alone, as a string or a one-element array.
(push 1) (assert (accepts-with "aud" (Present (JString "gates-oidc")))) (check-sat) (pop 1) ; string: sat
(push 1) (assert (accepts-with "aud" (Present (JStringArray (seq.unit "gates-oidc"))))) (check-sat) (pop 1) ; one-element array: sat
(push 1) (assert (accepts-with "aud" (Present (JStringArray (as seq.empty (Seq String)))))) (check-sat) (pop 1) ; empty array: unsat
(push 1) (assert (accepts-with "aud" (Present (JStringArray (seq.++ (seq.unit "gates-oidc") (seq.unit "other")))))) (check-sat) (pop 1) ; with another Audience: unsat
(push 1) (assert (accepts-with "aud" (Present (JStringArray (seq.unit ""))))) (check-sat) (pop 1) ; empty element: unsat
(push 1) (assert (accepts-with "aud" (Present (JString "sts.amazonaws.com")))) (check-sat) (pop 1) ; AWS Audience: unsat
(push 1) (assert (accepts-with "aud" (Present (JString "GATES-OIDC")))) (check-sat) (pop 1) ; different case: unsat
(push 1) (assert (accepts-with "aud" Absent)) (check-sat) (pop 1) ; missing: unsat
