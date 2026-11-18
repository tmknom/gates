; expect: sat sat sat
; trace:checks RFC7519-AUD-FORMAT RFC7519-AUD-SINGLE-STRING RFC7519-AUD-OPTIONAL RFC7519-AUD-INTENDED RFC7519-AUD-REJECT RFC8725-AUD-REQUIRED RFC8725-AUD-VALIDATE
; The audience clauses do not contradict each other: some Claims Set satisfies
; all of them with aud as an array, some with aud as a single string, and,
; when the Issuer does not issue for more than one recipient, some without aud.
(declare-const anyClaims Claims)
(declare-const recipient String)
(declare-const multiRecipientIssuer Bool)
(define-fun standards-audience ((c Claims)) Bool
  (and (std-aud-identifies c recipient)
       (std-aud-required c multiRecipientIssuer)
       (std-aud-validate c recipient multiRecipientIssuer)))
(push 1)
(assert (standards-audience anyClaims))
(assert multiRecipientIssuer)
(assert (std-aud-format (claim-value (select anyClaims "aud"))))
(check-sat)
(pop 1)
(push 1)
(assert (standards-audience anyClaims))
(assert multiRecipientIssuer)
(assert (std-aud-single-string (claim-value (select anyClaims "aud"))))
(check-sat)
(pop 1)
(push 1)
(assert (standards-audience anyClaims))
(assert (not multiRecipientIssuer))
(assert (std-aud-omitted anyClaims))
(check-sat)
(pop 1)
