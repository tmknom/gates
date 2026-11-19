; RFC 7518 JSON Web Algorithms: the clauses on accepting the none algorithm,
; which constrain the decoded JOSE Header. The clauses on keys and signature
; values are formalized with the signature model. The conventions of
; model/standards/ are in rfc7159.smt2.

(define-fun alg-none ((h Header)) Bool
  (= (get-member h "alg") (Present (JString "none"))))
; trace:formalizes RFC7518-NONE-CREATE
(define-fun std-none-create ((h Header)) Bool
  (alg-none h))
; supports is the Condition that the implementation supports Unsecured JWSs,
; and specified that the application specified, for this object, that no
; integrity protection is acceptable.
; trace:formalizes RFC7518-NONE-ACCEPT-SPECIFIED
(define-fun std-none-accept-specified ((h Header) (supports Bool) (specified Bool)) Bool
  (=> (and supports (not specified)) (not (alg-none h))))
; perObject is the application's indication, for this object, that an
; Unsecured JWS is acceptable.
; trace:formalizes RFC7518-NONE-PER-OBJECT
(define-fun std-none-per-object ((h Header) (perObject Bool)) Bool
  (=> (alg-none h) perObject))
; configured is the Condition negated: the implementation has been configured
; to accept an Unsecured JWS, rather than acting by default.
; trace:formalizes RFC7518-NONE-ACCEPT-DEFAULT
(define-fun std-none-accept-default ((h Header) (configured Bool)) Bool
  (=> (not configured) (not (alg-none h))))
; global tells that the application signals, for all objects at once, that
; an Unsecured JWS is acceptable.
; trace:formalizes RFC7518-NONE-GLOBAL
(define-fun std-none-global ((global Bool)) Bool
  (not global))
