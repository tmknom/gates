; expect: unsat unsat unsat unsat unsat sat sat sat sat sat sat sat
; EXAMPLES-NAMES (GOJ-CLAIM-NAME, GOJ-DYNAMIC-CLAIM, Verification Cases Claim name
; comparison, audit and dynamic Claims): aliases are rejected even with the correct name
; present; other names, audit Claims of any type and dynamic Claims are not.
(push 1) (assert (accepts-with "ISS" (Present (JString "https://token.actions.githubusercontent.com")))) (check-sat) (pop 1) ; ISS: unsat
(push 1) (assert (accepts-with "i\u{17f}\u{17f}" (Present (JString "https://token.actions.githubusercontent.com")))) (check-sat) (pop 1) ; iſſ: unsat
(push 1) (assert (accepts-with "i\u{17f}S" (Present (JString "https://token.actions.githubusercontent.com")))) (check-sat) (pop 1) ; iſS: unsat
(push 1) (assert (accepts-with "aUd" (Present (JString "gates-oidc")))) (check-sat) (pop 1) ; aUd: unsat
(push 1) (assert (accepts-with "NBF" (Present (JInteger 700)))) (check-sat) (pop 1) ; NBF: unsat
(push 1) (assert (accepts-with "i\u{df}" (Present (JString "x")))) (check-sat) (pop 1) ; iß: sat
(push 1) (assert (accepts-with "\u{130}ss" (Present (JString "x")))) (check-sat) (pop 1) ; İss: sat
(push 1) (assert (accepts-with "\u{131}ss" (Present (JString "x")))) (check-sat) (pop 1) ; ıss: sat
(push 1) (assert (accepts-with "actor" (Present (JInteger 42)))) (check-sat) (pop 1) ; audit Claim of another type: sat
(push 1) (assert (accepts-with "environment" (Present JNull))) (check-sat) (pop 1) ; audit Claim null: sat
(push 1) (assert (accepts-with "repo_property_pci_compliant" (Present (JString "true")))) (check-sat) (pop 1) ; dynamic Claim: sat
(push 1) (assert (accepts-with "new_claim" (Present JObject))) (check-sat) (pop 1) ; unknown nested Claim: sat
