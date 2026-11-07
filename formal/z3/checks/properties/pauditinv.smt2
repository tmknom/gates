(declare-const anyClaims Claims)
(declare-const changeName String)
(declare-const changeValue MaybeClaim)
; PAUDITINV: arbitrary mutation of any of the eight audit names
; cannot change the consulted CoreView.
(assert (audit-name changeName))
(assert (not (= (core-view anyClaims)
                (core-view (store anyClaims changeName changeValue)))))
(check-sat)
