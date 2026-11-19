; VALUES -- the decoded JOSE Header and JWT Claims Set, NOT raw bytes.
; Exactly one Member per decoded name. Absent differs from JNull.
; JInteger/JFraction/JExponent are lexical NUMBER categories assigned before
; this model. JStringArray represents a genuine all-string JSON array.
; Nested JSON is intentionally abstracted as JObject/JOtherArray.
; Strings are sequences of Unicode code points after JSON escaping is removed.
; No parser, duplicate-name detection or cryptographic proof.
; This file holds no clause of any layer; model/standards/, model/github/ and
; model/gates/ build on it without reading each other.
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
(declare-datatypes () ((Member Absent (Present (member-value JsonValue)))))
(define-sort JsonObject () (Array String Member))
(define-sort Claims () JsonObject)
(define-sort Header () JsonObject)
(define-fun get-member ((o JsonObject) (name String)) Member (select o name))
(define-fun string-member ((o JsonObject) (name String)) Bool
  (let ((m (get-member o name)))
    (and ((_ is Present) m) ((_ is JString) (member-value m)))))
(define-fun string-member-value ((o JsonObject) (name String)) String
  (string-value (member-value (get-member o name))))
(define-fun string-or-absent ((o JsonObject) (name String)) Bool
  (or (= (get-member o name) Absent) (string-member o name)))
(define-fun json-number ((v JsonValue)) Bool
  (or ((_ is JInteger) v) ((_ is JFraction) v) ((_ is JExponent) v)))
; Defined only for json-number.
(define-fun json-number-seconds ((v JsonValue)) Real
  (ite ((_ is JInteger) v) (to_real (integer-value v))
       (ite ((_ is JFraction) v) (fraction-value v) (exponent-value v))))
(define-fun ascii-text ((s String)) Bool
  (str.in_re s (re.* (re.range "\u{0000}" "\u{007F}"))))
(define-fun string-array-contains ((v JsonValue) (s String)) Bool
  (and ((_ is JStringArray) v)
       (seq.contains (array-strings v) (seq.unit s))))
