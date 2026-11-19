; expect: unsat
; trace:checks GOJ-HEADER-KEY-REFERENCE RFC8725-JKU-X5U-SSRF
; GATES keeps no list of allowed locations. No value of jku or x5u changes
; acceptance, so the URL either holds is never followed, which the clause
; asks when the URL is not on such a list.
(declare-const anyHeader Header)
(declare-const value Member)
(declare-const otherValue Member)
(declare-const name String)
(assert (or (= name "jku") (= name "x5u")))
(define-fun followed () Bool
  (not (= (gates-header-accepted (store anyHeader name value))
          (gates-header-accepted (store anyHeader name otherValue)))))
(assert (not (std-jku-x5u-ssrf followed false)))
(check-sat)
