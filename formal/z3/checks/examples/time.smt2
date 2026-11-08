; expect: unsat sat sat unsat unsat unsat unsat unsat unsat sat unsat sat sat unsat sat unsat unsat unsat
; EXAMPLES-TIME (GOJ-EXP, GOJ-IAT, GOJ-NBF, GOJ-TIME, Verification Cases NumericDate): the model
; accepts and rejects the representative times as gates-oidc-jwt-claims.md states. now=1000.
(push 1) (assert (accepts-with "exp" (Present (JInteger 1000)))) (check-sat) (pop 1) ; exp=now: unsat
(push 1) (assert (accepts-with "exp" (Present (JInteger 1001)))) (check-sat) (pop 1) ; exp=now+1: sat
(push 1) (assert (accepts-with "exp" (Present (JInteger 253402300799)))) (check-sat) (pop 1) ; exp at upper limit: sat
(push 1) (assert (accepts-with "exp" (Present (JInteger 253402300800)))) (check-sat) (pop 1) ; exp above limit: unsat
(push 1) (assert (accepts-with "exp" (Present (JInteger (- 1))))) (check-sat) (pop 1) ; exp negative: unsat
(push 1) (assert (accepts-with "exp" (Present (JFraction 1001.0)))) (check-sat) (pop 1) ; exp fraction: unsat
(push 1) (assert (accepts-with "exp" (Present (JExponent 1001.0)))) (check-sat) (pop 1) ; exp exponent: unsat
(push 1) (assert (accepts-with "exp" (Present (JString "1001")))) (check-sat) (pop 1) ; exp numeric string: unsat
(push 1) (assert (accepts-with "exp" Absent)) (check-sat) (pop 1) ; exp missing: unsat
(push 1) (assert (accepts-with "iat" (Present (JInteger 1300)))) (check-sat) (pop 1) ; iat=now+300: sat
(push 1) (assert (accepts-with "iat" (Present (JInteger 1301)))) (check-sat) (pop 1) ; iat=now+301: unsat
(push 1) (assert (accepts-with "iat" (Present (JInteger 0)))) (check-sat) (pop 1) ; iat at lower limit: sat
(push 1) (assert (accepts-with "nbf" (Present (JInteger 1300)))) (check-sat) (pop 1) ; nbf=now+300: sat
(push 1) (assert (accepts-with "nbf" (Present (JInteger 1301)))) (check-sat) (pop 1) ; nbf=now+301: unsat
(push 1) (assert (accepts-with "nbf" Absent)) (check-sat) (pop 1) ; nbf missing: sat
(push 1) (assert (accepts-with "nbf" (Present JNull))) (check-sat) (pop 1) ; nbf null: unsat
(push 1) (assert (accepts-with "iat" Absent)) (check-sat) (pop 1) ; iat missing: unsat
(push 1) (assert (accepts-with "iat" (Present (JString "1000")))) (check-sat) (pop 1) ; iat numeric string: unsat
