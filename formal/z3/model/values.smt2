; VALUES -- decoded JSON Claims Set, NOT raw bytes.
; Exactly one Map value per decoded Claim name. Missing differs from JNull.
; JInteger/JFraction/JExponent are lexical NUMBER categories assigned before
; this model. JStringArray represents a genuine all-string JSON array.
; Nested JSON is intentionally abstracted as JObject/JOtherArray.
; No parser, duplicate-name detection, cryptographic or Unicode folding proof.
(set-logic ALL)
(set-option :produce-models true)
(set-option :timeout 8000)
(declare-datatypes () ((JsonValue
  JNull
  (JBoolean (bool-value Bool))
  (JString (string-value String))
  (JInteger (integer-value Int))
  (JFraction (fraction-value Real))
  (JExponent (exponent-value Real))
  (JStringArray (array-strings (Seq String)))
  JOtherArray
  JObject)))
(declare-datatypes () ((MaybeClaim Absent (Present (claim-value JsonValue)))))
(define-sort Claims () (Array String MaybeClaim))
(define-fun get-claim ((c Claims) (name String)) MaybeClaim (select c name))
(define-fun string-is ((c Claims) (name String) (expected String)) Bool
  (= (get-claim c name) (Present (JString expected))))
(define-fun integer-in-range ((c Claims) (name String)) Bool
  (let ((m (get-claim c name)))
    (and ((_ is Present) m)
         (let ((v (claim-value m)))
           (and ((_ is JInteger) v)
                (<= 0 (integer-value v))
                (<= (integer-value v) 253402300799))))))
(define-fun integer-value-is ((c Claims) (name String) (value Int)) Bool
  (= (get-claim c name) (Present (JInteger value))))
(define-fun string-claim ((c Claims) (name String)) Bool
  (let ((m (get-claim c name)))
    (and ((_ is Present) m) ((_ is JString) (claim-value m)))))
(define-fun string-claim-value ((c Claims) (name String)) String
  (string-value (claim-value (get-claim c name))))
(define-fun numeric-date-typed ((m MaybeClaim)) Bool
  (and ((_ is Present) m)
       (let ((v (claim-value m)))
         (or ((_ is JInteger) v) ((_ is JFraction) v) ((_ is JExponent) v)))))
(define-fun numeric-date-value ((m MaybeClaim)) Real
  (let ((v (claim-value m)))
    (ite ((_ is JInteger) v) (to_real (integer-value v))
         (ite ((_ is JFraction) v) (fraction-value v) (exponent-value v)))))
; Accepted nonzero decimal digits spelling, max signed int64.
(define-fun positive-int64-text ((s String)) Bool
  (and (str.in_re s
        (re.++ (re.range "1" "9") (re.* (re.range "0" "9"))))
       (or (< (str.len s) 19)
           (and (= (str.len s) 19)
                (str.<= s "9223372036854775807")))))
(define-fun canonical-id-claim ((c Claims) (name String)) Bool
  (and (string-claim c name)
       (positive-int64-text (string-claim-value c name))))
(define-fun ascii-text ((s String)) Bool
  (str.in_re s (re.* (re.range "\u{0000}" "\u{007F}"))))

; Exact registry of GATES-used names. Noncore names can be audit/unused/unknown.
(define-fun core-name ((name String)) Bool
  (or (= name "iss") (= name "aud") (= name "exp") (= name "iat")
      (= name "nbf") (= name "sub")
      (= name "repository_owner_id") (= name "repository_id")
      (= name "run_id") (= name "run_attempt")))
(define-fun audit-name ((name String)) Bool
  (or (= name "actor") (= name "actor_id") (= name "event_name")
      (= name "ref") (= name "sha") (= name "workflow_ref")
      (= name "job_workflow_ref") (= name "environment")))
; 18 Discovery-known unused names. Unrecognized names including repo_property_*
; are also ignored for acceptance (subject to raw JSON validity and aliases).
(define-fun known-unused-name ((name String)) Bool
  (or (= name "jti") (= name "repository") (= name "repository_owner")
      (= name "enterprise") (= name "enterprise_id") (= name "run_number")
      (= name "workflow") (= name "workflow_sha")
      (= name "head_ref") (= name "base_ref") (= name "ref_type")
      (= name "ref_protected") (= name "environment_node_id")
      (= name "job_workflow_sha") (= name "repository_visibility")
      (= name "runner_environment") (= name "issuer_scope") (= name "check_run_id")))
(define-fun discovery-known-name ((name String)) Bool
  (or (core-name name) (audit-name name) (known-unused-name name)))
