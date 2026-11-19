; expect: sat sat
; trace:checks GOJ-ISS GOJ-AUD GOJ-EXP GOJ-IAT GOJ-NBF GOJ-TIME-EXP GOJ-TIME-IAT GOJ-TIME-NBF GOJ-SUB-FORMAT GOJ-SUB-LENGTH GOJ-ID-FORMAT GOJ-ID-MATCH GOJ-REPOSITORY-ID GOJ-REPOSITORY-OWNER-ID GOJ-RUN-ID GOJ-RUN-ATTEMPT GOJ-CLAIM-NAME-MATCH GOJ-CLAIM-NAME-ALIAS GOJ-HEADER-ALG GOJ-HEADER-KID GOJ-HEADER-CTY GOJ-HEADER-CRIT GOJ-HEADER-B64 GOJ-HEADER-KEY-REFERENCE GOJ-HEADER-UNKNOWN
; An accepted Claims Set and an accepted JOSE Header exist, so the GATES
; acceptance predicates do not contradict each other and the universal checks
; do not hold vacuously.
(push 1)
(assert (gates-claims-accepted sample 1000 "gates-oidc"))
(check-sat)
(pop 1)
(push 1)
(assert (gates-header-accepted sample-header))
(check-sat)
(pop 1)
