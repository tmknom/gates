; expect: unsat
; trace:checks GOJ-HEADER-UNKNOWN RFC7515-HEADER-EXTENSION-UNKNOWN-IGNORE
; GATES understands the Header Parameters its rules read, b64 and nonce among
; them. Any other Header Parameter that RFC 7515 does not define leaves
; acceptance as it is without the parameter, whatever its value. An accepted
; Header has no crit, so no such parameter is listed in crit.
(declare-const anyHeader Header)
(declare-const name String)
(declare-const value JsonValue)
(assert (not (std-jws-extension-unknown-ignore
               name false (header-consulted-name name)
               (gates-header-accepted (store anyHeader name (Present value)))
               (gates-header-accepted (store anyHeader name Absent)))))
(check-sat)
