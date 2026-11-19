; expect: unsat
; trace:checks GOJ-ID-FORMAT GH-OIDC-CLAIM-TYPE-OBSERVED
; In every issuance the Observed clauses allow, the ID Claims that are present
; are strings of digits, so GATES accepts them once the digits have the
; GOJ-ID-FORMAT spelling (GH-OIDC-CLAIM-ID-FORMAT). The Documented clauses do
; not give the JSON type (GH-OIDC-CLAIM-TYPE), which the Observed one does.
(define-fun spelled ((n String)) Bool
  (=> (string-member issued n) (gates-id-text (string-member-value issued n))))
(define-fun accepted ((n String)) Bool
  (=> ((_ is Present) (select issued n)) (gates-id-claim issued n)))
(assert observed-issuance)
(assert (spelled "repository_owner_id"))
(assert (spelled "repository_id"))
(assert (spelled "run_id"))
(assert (spelled "run_attempt"))
(assert (not (and (accepted "repository_owner_id") (accepted "repository_id")
                  (accepted "run_id") (accepted "run_attempt"))))
(check-sat)
