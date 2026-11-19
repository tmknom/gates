; expect: unsat
; checks/consistency/standards-audience.smt2 loses its third witness when
; RFC8725-AUD-REQUIRED is misread as bad-std-aud-required, which asks for aud
; whatever the Issuer: no Claims Set then omits aud as RFC7519-AUD-OPTIONAL
; allows, so the consistency check detects the misreading.
(declare-const anyClaims Claims)
(assert (bad-std-aud-required anyClaims false))
(assert (std-aud-omitted anyClaims))
(check-sat)
