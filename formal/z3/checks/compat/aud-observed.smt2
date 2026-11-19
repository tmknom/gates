; expect: unsat
; trace:checks GOJ-AUD GH-OIDC-AUD-CUSTOM-STRING
; GATES accepts the aud of every issuance the Observed clauses allow as well,
; for a GATES Audience that is not a URL, as in the records: aud is then the
; requested Audience as a string. For a URL, only aud-documented.smt2 applies,
; with presence as its premise.
(assert observed-issuance)
(assert (> (str.len expected) 0))
(assert (not (str.contains expected "://")))
(assert (not (gates-aud issued expected)))
(check-sat)
