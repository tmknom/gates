; expect: unsat
; trace:checks GOJ-AUD RFC8725-AUD-VALIDATE
; Every Claims Set GATES accepts for an Audience contains aud associated with
; that Audience, whether or not the Issuer issues for more than one recipient.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(declare-const multiRecipientIssuer Bool)
(assert (gates-current anyClaims arbitraryClock arbitraryAudience))
(assert (not (std-aud-validate anyClaims arbitraryAudience multiRecipientIssuer)))
(check-sat)
