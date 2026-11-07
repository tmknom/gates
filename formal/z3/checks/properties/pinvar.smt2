(declare-const anyClaims Claims)
(declare-const changeName String)
(declare-const changeValue MaybeClaim)
; PINVAR: no change to any consulted field under a noncore/nonalias update.
(assert (not (core-name changeName)))
(assert (not (recognized-alias-example changeName)))
(assert (not (= (core-view anyClaims)
                (core-view (store anyClaims changeName changeValue)))))
(check-sat)
