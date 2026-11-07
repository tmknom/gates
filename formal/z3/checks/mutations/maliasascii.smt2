; expect: sat
; MALIASASCII: PALIAS has a counterexample against bad-ascii-aliases-only,
; which misses the aliases with U+017F. The witness is the accepted sample
; with the alias "iſs" added; the value stays arbitrary.
(declare-const aliasValue JsonValue)
(assert (claim-alias "i\u{17f}s"))
(assert (bad-ascii-aliases-only (store sample "i\u{17f}s" (Present aliasValue))
                                1000 "gates-oidc"))
(check-sat)
