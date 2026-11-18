; expect: sat sat unsat unsat unsat unsat unsat unsat unsat unsat unsat unsat unsat unsat sat sat sat sat unsat sat unsat sat sat
; trace:checks GOJ-HEADER-ALG GOJ-HEADER-KID GOJ-HEADER-TYP GOJ-HEADER-CTY GOJ-HEADER-CRIT GOJ-HEADER-B64 GOJ-HEADER-KEY-REFERENCE GOJ-HEADER-UNKNOWN
; Verification Cases of the JOSE Header: the model accepts RS256 with a
; non-empty kid, ignores typ, the key references it does not read and unknown
; parameters, and rejects none, other algorithms, a missing or empty kid, cty,
; crit, b64, an unreadable jwk or x5c and a nonce that is not a string.
(push 1) (assert (gates-header-accepted sample-header)) (check-sat) (pop 1) ; RS256 and kid: sat
(push 1) (assert (header-accepts-with "typ" (Present (JString "JWT")))) (check-sat) (pop 1) ; typ JWT: sat
(push 1) (assert (header-accepts-with "alg" (Present (JString "none")))) (check-sat) (pop 1) ; alg none: unsat
(push 1) (assert (header-accepts-with "alg" (Present (JString "PS256")))) (check-sat) (pop 1) ; alg PS256: unsat
(push 1) (assert (header-accepts-with "alg" (Present (JString "rs256")))) (check-sat) (pop 1) ; alg in another case: unsat
(push 1) (assert (header-accepts-with "alg" Absent)) (check-sat) (pop 1) ; alg missing: unsat
(push 1) (assert (header-accepts-with "kid" Absent)) (check-sat) (pop 1) ; kid missing: unsat
(push 1) (assert (header-accepts-with "kid" (Present (JString "")))) (check-sat) (pop 1) ; kid empty: unsat
(push 1) (assert (header-accepts-with "kid" (Present (JInteger 1)))) (check-sat) (pop 1) ; kid number: unsat
(push 1) (assert (header-accepts-with "cty" (Present (JString "JWT")))) (check-sat) (pop 1) ; cty JWT: unsat
(push 1) (assert (header-accepts-with "cty" (Present (JString "jwt")))) (check-sat) (pop 1) ; cty jwt: unsat
(push 1) (assert (header-accepts-with "crit" (Present (JStringArray (seq.unit "exp"))))) (check-sat) (pop 1) ; crit: unsat
(push 1) (assert (header-accepts-with "b64" (Present (JBoolean true)))) (check-sat) (pop 1) ; b64 true: unsat
(push 1) (assert (header-accepts-with "b64" (Present (JBoolean false)))) (check-sat) (pop 1) ; b64 false: unsat
(push 1) (assert (header-accepts-with "typ" (Present (JString "at+jwt")))) (check-sat) (pop 1) ; typ of another media type: sat
(push 1) (assert (header-accepts-with "x5t" (Present (JString "thumbprint")))) (check-sat) (pop 1) ; x5t: sat
(push 1) (assert (header-accepts-with "jku" (Present (JString "https://example.com/jwks")))) (check-sat) (pop 1) ; jku not fetched: sat
(push 1) (assert (header-accepts-with "jwk" (Present JObject))) (check-sat) (pop 1) ; jwk object: sat
(push 1) (assert (header-accepts-with "jwk" (Present (JString "key")))) (check-sat) (pop 1) ; jwk not an object: unsat
(push 1) (assert (header-accepts-with "x5c" (Present (JStringArray (seq.unit "MIIC"))))) (check-sat) (pop 1) ; x5c array: sat
(push 1) (assert (header-accepts-with "nonce" (Present (JInteger 1)))) (check-sat) (pop 1) ; nonce number: unsat
(push 1) (assert (header-accepts-with "nonce" (Present (JString "n")))) (check-sat) (pop 1) ; nonce string: sat
(push 1) (assert (header-accepts-with "unknown" (Present JObject))) (check-sat) (pop 1) ; unknown parameter: sat
