; expect: sat
; trace:checks GOJ-HEADER-KID
; GH-OIDC-HEADER-DOCUMENTED describes kid without its value or type. A Header
; with an empty kid is allowed, which GATES rejects: GATES relies on the
; Published and Observed clauses to accept kid.
(assert documented-issuance)
(assert (= (select issuedHeader "kid") (Present (JString ""))))
(assert (not (gates-header-kid issuedHeader)))
(check-sat)
