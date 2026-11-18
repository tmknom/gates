; expect: unsat
; trace:checks GOJ-TIME-EXP GOJ-TIME-IAT GOJ-TIME-NBF
; The clocks at which a Claims Set is accepted form an interval. A set
; accepted at two clocks is accepted at every clock between them, so the time
; rules leave no gap in a validity period.
(declare-const anyClaims Claims)
(declare-const arbitraryAudience String)
(declare-const early Int)
(declare-const middle Int)
(declare-const late Int)
(assert (<= early middle))
(assert (<= middle late))
(assert (gates-claims-accepted anyClaims early arbitraryAudience))
(assert (gates-claims-accepted anyClaims late arbitraryAudience))
(assert (not (gates-claims-accepted anyClaims middle arbitraryAudience)))
(check-sat)
