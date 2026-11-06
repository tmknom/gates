; Model only the optional audit extraction/normalization rule.
; Source: jwt.md §2.38.4 (string extraction); domain.md §3.3 (empty omitted).
; Extraction eligibility: a string, even empty, can be read.
(define-fun audit-extract-string ((c Claims) (name String)) Bool
  (and (audit-name name) (string-claim c name)))
; Normalization: empty, wrong type or absent -> no audit attribute.
(define-fun audit-attribute-present ((c Claims) (name String)) Bool
  (and (audit-extract-string c name)
       (> (str.len (string-claim-value c name)) 0)))
