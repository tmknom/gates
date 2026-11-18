; expect: unsat
; trace:checks GOJ-AUD RFC8725-AUD-REQUIRED
; Every Claims Set GATES accepts contains aud, whether or not the Issuer issues
; for more than one recipient.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(assert (gates-claims-accepted anyClaims arbitraryClock arbitraryAudience))
(declare-const multiRecipientIssuer Bool)
(assert (not (std-aud-required anyClaims multiRecipientIssuer)))
(check-sat)
