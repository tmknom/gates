; RFC 7159 JSON: the clause on string comparison.
; Each file under model/standards/ formalizes the clauses of one source, is
; written from its clauses alone, and holds no GATES value (Issuer, Audience,
; leeway, algorithm). A MAY or OPTIONAL clause is the predicate of the choice it
; permits, a value the standard leaves open is a parameter, and a Condition is a
; Bool parameter. A clause whose actor issues or produces the JWT constrains what
; is issued; a clause whose actor receives or processes it constrains what may
; be accepted.

; Strings in the model are code point sequences after the JSON escaping is
; removed. For valid Unicode strings, comparing code units agrees with
; comparing code points, so the comparison is equality of the sequences.
; trace:formalizes RFC7159-STRING-COMPARE
(define-fun std-json-string-equal ((a String) (b String)) Bool
  (= a b))
