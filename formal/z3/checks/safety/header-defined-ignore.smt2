; expect: unsat
; trace:checks GOJ-HEADER-TYP GOJ-HEADER-KEY-REFERENCE RFC7515-HEADER-DEFINED-UNKNOWN-IGNORE
; GATES understands the Header Parameters its rules read. Any other Header
; Parameter that RFC 7515 defines, typ, jku, x5u, x5t and x5t#S256 among them,
; leaves acceptance as it is without the parameter, whatever its value.
(declare-const anyHeader Header)
(declare-const name String)
(declare-const value JsonValue)
(assert (not (std-jws-defined-unknown-ignore
               name (header-consulted-name name)
               (gates-header-accepted (store anyHeader name (Present value)))
               (gates-header-accepted (store anyHeader name Absent)))))
(check-sat)
