; expect: unsat
; trace:checks GOJ-AUD RFC7519-STRING-COMPARE RFC7519-STRINGORURI-COMPARE
; Every Claims Set GATES accepts for an Audience has, as its only aud value,
; a string equal to that Audience under the comparison of both clauses, which
; neither folds case nor normalizes.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(assert (gates-claims-accepted anyClaims arbitraryClock arbitraryAudience))
(define-fun aud-value () JsonValue (member-value (select anyClaims "aud")))
(define-fun only-value () String
  (ite ((_ is JString) aud-value) (string-value aud-value) (seq.nth (array-strings aud-value) 0)))
(assert (not (and (or ((_ is JString) aud-value)
                      (and ((_ is JStringArray) aud-value) (= (seq.len (array-strings aud-value)) 1)))
                  (std-jwt-string-equal only-value arbitraryAudience)
                  (std-stringoruri-equal only-value arbitraryAudience))))
(check-sat)
