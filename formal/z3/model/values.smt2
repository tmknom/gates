; VALUES -- decoded JSON Claims Set, NOT raw bytes.
; Exactly one Map value per decoded Claim name. Missing differs from JNull.
; JInteger/JFraction/JExponent are lexical NUMBER categories assigned before
; this model. JStringArray represents a genuine all-string JSON array.
; Nested JSON is intentionally abstracted as JObject/JOtherArray.
; No parser, duplicate-name detection or cryptographic proof.
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

; Exact registry of GATES-used names. Other names are audit/unused/unknown.
(define-fun core-name ((name String)) Bool
  (or (= name "iss") (= name "aud") (= name "exp") (= name "iat")
      (= name "nbf") (= name "sub")
      (= name "repository_owner_id") (= name "repository_id")
      (= name "run_id") (= name "run_attempt")))
(define-fun audit-name ((name String)) Bool
  (or (= name "actor") (= name "actor_id") (= name "event_name")
      (= name "ref") (= name "sha") (= name "workflow_ref")
      (= name "job_workflow_ref") (= name "environment")))

; Claim name alias (jwt.md §2.38.4): a name other than iss/aud/exp/iat/nbf
; that folds to the same string under Unicode simple case folding
; (CaseFolding.txt status C and S). For the letters of these five names,
; each ASCII letter folds with its other case, and s/S/U+017F fold together.
(define-fun simple-fold-letter ((c String) (base String)) Bool
  (or (= c base)
      (and (= base "a") (= c "A"))
      (and (= base "b") (= c "B"))
      (and (= base "d") (= c "D"))
      (and (= base "e") (= c "E"))
      (and (= base "f") (= c "F"))
      (and (= base "i") (= c "I"))
      (and (= base "n") (= c "N"))
      (and (= base "p") (= c "P"))
      (and (= base "s") (or (= c "S") (= c "\u{17f}")))
      (and (= base "t") (= c "T"))
      (and (= base "u") (= c "U"))
      (and (= base "x") (= c "X"))))
(define-fun alias-of ((n String) (name String)) Bool
  (and (not (= n name))
       (= (str.len n) 3)
       (simple-fold-letter (str.at n 0) (str.at name 0))
       (simple-fold-letter (str.at n 1) (str.at name 1))
       (simple-fold-letter (str.at n 2) (str.at name 2))))
(define-fun claim-alias ((n String)) Bool
  (or (alias-of n "iss") (alias-of n "aud") (alias-of n "exp")
      (alias-of n "iat") (alias-of n "nbf")))
; Every name the acceptance predicate reads. PCONGRUENCE checks that equal
; values under these names imply equal acceptance.
(define-fun consulted-name ((n String)) Bool
  (or (core-name n) (claim-alias n)))
