; GATES, CLAIMS SET -- the GOJ acceptance rules on the decoded Claims Set.
; Source of truth: docs/design/oidc-jwt-claims.md. Each predicate is one
; GOJ decision; gates-claims-accepted is their conjunction.

; GATES reads a Claim only under the name that equals it exactly.
; trace:formalizes GOJ-CLAIM-NAME-MATCH
(define-fun gates-claim ((c Claims) (name String)) Member
  (select c name))
(define-fun gates-string ((c Claims) (name String)) Bool
  (let ((m (gates-claim c name)))
    (and ((_ is Present) m) ((_ is JString) (member-value m)))))
(define-fun gates-string-value ((c Claims) (name String)) String
  (string-value (member-value (gates-claim c name))))

(define-fun gates-fixed-issuer () String "https://token.actions.githubusercontent.com")
; trace:formalizes GOJ-ISS
(define-fun gates-iss ((c Claims)) Bool
  (= (gates-claim c "iss") (Present (JString gates-fixed-issuer))))

; trace:formalizes GOJ-AUD
(define-fun gates-aud ((c Claims) (expected String)) Bool
  (and (> (str.len expected) 0)
       (let ((m (gates-claim c "aud")))
         (and ((_ is Present) m)
              (let ((v (member-value m)))
                (or (and ((_ is JString) v) (= (string-value v) expected))
                    (and ((_ is JStringArray) v)
                         (= (array-strings v) (seq.unit expected)))))))))

(define-fun integer-in-range ((c Claims) (name String)) Bool
  (let ((m (gates-claim c name)))
    (and ((_ is Present) m)
         (let ((v (member-value m)))
           (and ((_ is JInteger) v)
                (<= 0 (integer-value v))
                (<= (integer-value v) 253402300799))))))
; trace:formalizes GOJ-EXP
(define-fun gates-exp ((c Claims)) Bool
  (integer-in-range c "exp"))
; trace:formalizes GOJ-IAT
(define-fun gates-iat ((c Claims)) Bool
  (integer-in-range c "iat"))
; trace:formalizes GOJ-NBF
(define-fun gates-nbf ((c Claims)) Bool
  (or (= (gates-claim c "nbf") Absent)
      (integer-in-range c "nbf")))
; Defined only for a Claim that gates-exp, gates-iat or gates-nbf admits.
(define-fun claim-integer ((c Claims) (name String)) Int
  (integer-value (member-value (gates-claim c name))))
; L is 300 seconds.
(define-fun gates-leeway () Int 300)
; The time predicates are meaningful only together with gates-exp, gates-iat
; and gates-nbf, which admit the integers they read.
; trace:formalizes GOJ-TIME-EXP
(define-fun gates-time-exp ((c Claims) (now Int)) Bool
  (< now (claim-integer c "exp")))
; trace:formalizes GOJ-TIME-IAT
(define-fun gates-time-iat ((c Claims) (now Int)) Bool
  (<= (claim-integer c "iat") (+ now gates-leeway)))
; trace:formalizes GOJ-TIME-NBF
(define-fun gates-time-nbf ((c Claims) (now Int)) Bool
  (or (= (gates-claim c "nbf") Absent)
      (<= (claim-integer c "nbf") (+ now gates-leeway))))
; The iat and nbf conditions, apart so that mutations can vary exp.
(define-fun gates-time-not-ahead ((c Claims) (now Int)) Bool
  (and (gates-time-iat c now) (gates-time-nbf c now)))
(define-fun gates-time ((c Claims) (now Int)) Bool
  (and (gates-time-exp c now) (gates-time-not-ahead c now)))

; Accepted nonzero decimal digits spelling, max signed int64.
; trace:formalizes GOJ-ID-FORMAT
(define-fun gates-id-text ((s String)) Bool
  (and (str.in_re s (re.++ (re.range "1" "9") (re.* (re.range "0" "9"))))
       (<= (str.to_int s) 9223372036854775807)))
(define-fun gates-id-claim ((c Claims) (name String)) Bool
  (and (gates-string c name) (gates-id-text (gates-string-value c name))))

; Deterministic sub parser over decoded strings. Split at first '/' after
; 'repo:' and first ':' after it, so a repository name may contain '/' and an
; owner name may contain ':'. Each part is NAME@ID.
(define-fun sub-text ((c Claims)) String (gates-string-value c "sub"))
(define-fun sub-slash ((c Claims)) Int (str.indexof (sub-text c) "/" 5))
(define-fun sub-colon ((c Claims)) Int (str.indexof (sub-text c) ":" (+ (sub-slash c) 1)))
(define-fun sub-owner-part ((c Claims)) String
  (str.substr (sub-text c) 5 (- (sub-slash c) 5)))
(define-fun sub-repository-part ((c Claims)) String
  (str.substr (sub-text c) (+ (sub-slash c) 1) (- (sub-colon c) (sub-slash c) 1)))
(define-fun part-at ((p String)) Int (str.indexof p "@" 0))
(define-fun part-id ((p String)) String
  (str.substr p (+ (part-at p) 1) (- (str.len p) (part-at p) 1)))
(define-fun name-at-id ((p String)) Bool
  (and (> (part-at p) 0)
       (= (str.indexof p "@" (+ (part-at p) 1)) (- 1))
       (gates-id-text (part-id p))))
(define-fun sub-owner-id ((c Claims)) String (part-id (sub-owner-part c)))
(define-fun sub-repository-id ((c Claims)) String (part-id (sub-repository-part c)))
; Both IDs follow GOJ-ID-FORMAT.
; trace:formalizes GOJ-SUB-FORMAT
(define-fun gates-sub-format ((c Claims)) Bool
  (and (gates-string c "sub")
       (str.prefixof "repo:" (sub-text c))
       (> (sub-slash c) 5)
       (> (sub-colon c) (+ (sub-slash c) 1))
       (name-at-id (sub-owner-part c))
       (name-at-id (sub-repository-part c))))
; trace:formalizes GOJ-SUB-LENGTH
(define-fun gates-sub-length ((c Claims)) Bool
  (and (gates-string c "sub")
       (ascii-text (sub-text c))
       (<= (str.len (sub-text c)) 255)))
; Defined only for a sub that gates-sub-format admits.
; trace:formalizes GOJ-ID-MATCH
(define-fun gates-id-match ((c Claims)) Bool
  (and (gates-string c "repository_owner_id")
       (gates-string c "repository_id")
       (= (gates-string-value c "repository_owner_id") (sub-owner-id c))
       (= (gates-string-value c "repository_id") (sub-repository-id c))))

; trace:formalizes GOJ-REPOSITORY-ID
(define-fun gates-repository-id ((c Claims)) Bool
  (gates-id-claim c "repository_id"))
; trace:formalizes GOJ-REPOSITORY-OWNER-ID
(define-fun gates-repository-owner-id ((c Claims)) Bool
  (gates-id-claim c "repository_owner_id"))
; trace:formalizes GOJ-RUN-ID
(define-fun gates-run-id ((c Claims)) Bool
  (gates-id-claim c "run_id"))
; GOJ-ID-FORMAT rejects "0", so a positive number of attempts is accepted.
; trace:formalizes GOJ-RUN-ATTEMPT
(define-fun gates-run-attempt ((c Claims)) Bool
  (gates-id-claim c "run_attempt"))

; Claim name alias (GOJ-CLAIM-NAME-ALIAS): a name other than iss/aud/exp/iat/nbf
; that folds to the same string under Unicode simple case folding
; (CaseFolding.txt status C and S). For the letters of these five names,
; each ASCII letter folds with its other case, and s/S/U+017F fold together.
(define-fun simple-fold-letter ((c String) (base String)) Bool
  (or (= c base)
      (and (= base "a") (= c "A"))
      (and (= base "b") (= c "B"))
      (and (= base "d") (= c "D"))
      (and (= base "e") (= c "E"))
      (and (= base "f") (= c "F"))
      (and (= base "i") (= c "I"))
      (and (= base "n") (= c "N"))
      (and (= base "p") (= c "P"))
      (and (= base "s") (or (= c "S") (= c "\u{17f}")))
      (and (= base "t") (= c "T"))
      (and (= base "u") (= c "U"))
      (and (= base "x") (= c "X"))))
(define-fun alias-of ((n String) (name String)) Bool
  (and (not (= n name))
       (= (str.len n) 3)
       (simple-fold-letter (str.at n 0) (str.at name 0))
       (simple-fold-letter (str.at n 1) (str.at name 1))
       (simple-fold-letter (str.at n 2) (str.at name 2))))
(define-fun claim-alias ((n String)) Bool
  (or (alias-of n "iss") (alias-of n "aud") (alias-of n "exp")
      (alias-of n "iat") (alias-of n "nbf")))
; The 45 names that claim-alias holds for, listed so that acceptance reads only
; fixed names and needs no quantifier (Z3 answers unknown with one).
; checks/properties/alias.smt2 shows every claim-alias name is listed, and
; checks/safety/claim-name-match.smt2 shows no other name affects acceptance,
; so this list and claim-alias are the same set.
; trace:formalizes GOJ-CLAIM-NAME-ALIAS
(define-fun gates-aliases-absent ((c Claims)) Bool
  (and
    (= (gates-claim c "isS") Absent) (= (gates-claim c "is\u{17f}") Absent)
    (= (gates-claim c "iSs") Absent) (= (gates-claim c "iSS") Absent)
    (= (gates-claim c "iS\u{17f}") Absent) (= (gates-claim c "i\u{17f}s") Absent)
    (= (gates-claim c "i\u{17f}S") Absent) (= (gates-claim c "i\u{17f}\u{17f}") Absent)
    (= (gates-claim c "Iss") Absent) (= (gates-claim c "IsS") Absent)
    (= (gates-claim c "Is\u{17f}") Absent) (= (gates-claim c "ISs") Absent)
    (= (gates-claim c "ISS") Absent) (= (gates-claim c "IS\u{17f}") Absent)
    (= (gates-claim c "I\u{17f}s") Absent) (= (gates-claim c "I\u{17f}S") Absent)
    (= (gates-claim c "I\u{17f}\u{17f}") Absent)
    (= (gates-claim c "auD") Absent) (= (gates-claim c "aUd") Absent)
    (= (gates-claim c "aUD") Absent) (= (gates-claim c "Aud") Absent)
    (= (gates-claim c "AuD") Absent) (= (gates-claim c "AUd") Absent)
    (= (gates-claim c "AUD") Absent)
    (= (gates-claim c "exP") Absent) (= (gates-claim c "eXp") Absent)
    (= (gates-claim c "eXP") Absent) (= (gates-claim c "Exp") Absent)
    (= (gates-claim c "ExP") Absent) (= (gates-claim c "EXp") Absent)
    (= (gates-claim c "EXP") Absent)
    (= (gates-claim c "iaT") Absent) (= (gates-claim c "iAt") Absent)
    (= (gates-claim c "iAT") Absent) (= (gates-claim c "Iat") Absent)
    (= (gates-claim c "IaT") Absent) (= (gates-claim c "IAt") Absent)
    (= (gates-claim c "IAT") Absent)
    (= (gates-claim c "nbF") Absent) (= (gates-claim c "nBf") Absent)
    (= (gates-claim c "nBF") Absent) (= (gates-claim c "Nbf") Absent)
    (= (gates-claim c "NbF") Absent) (= (gates-claim c "NBf") Absent)
    (= (gates-claim c "NBF") Absent)))

; Exact registry of the names the required and optional-verified rules read.
(define-fun core-name ((n String)) Bool
  (or (= n "iss") (= n "aud") (= n "exp") (= n "iat") (= n "nbf") (= n "sub")
      (= n "repository_owner_id") (= n "repository_id")
      (= n "run_id") (= n "run_attempt")))
; Every name the acceptance predicate reads.
(define-fun consulted-name ((n String)) Bool
  (or (core-name n) (claim-alias n)))
; The Claims GATES does not use. checks/properties/claim-unused.smt2 shows that
; acceptance consults none of them, and with it
; checks/safety/claim-unknown-ignore.smt2 that no such name changes acceptance.
; trace:formalizes GOJ-JTI GOJ-REPOSITORY GOJ-REPOSITORY-OWNER GOJ-ENTERPRISE GOJ-ENTERPRISE-ID GOJ-RUN-NUMBER GOJ-WORKFLOW GOJ-WORKFLOW-SHA GOJ-HEAD-REF GOJ-BASE-REF GOJ-REF-TYPE GOJ-REF-PROTECTED GOJ-ENVIRONMENT-NODE-ID GOJ-JOB-WORKFLOW-SHA GOJ-REPOSITORY-VISIBILITY GOJ-RUNNER-ENVIRONMENT GOJ-ISSUER-SCOPE GOJ-CHECK-RUN-ID
(define-fun gates-unused-name ((n String)) Bool
  (or (= n "jti") (= n "repository") (= n "repository_owner")
      (= n "enterprise") (= n "enterprise_id") (= n "run_number")
      (= n "workflow") (= n "workflow_sha") (= n "head_ref") (= n "base_ref")
      (= n "ref_type") (= n "ref_protected") (= n "environment_node_id")
      (= n "job_workflow_sha") (= n "repository_visibility")
      (= n "runner_environment") (= n "issuer_scope") (= n "check_run_id")))
; The audit Claims, which acceptance does not read. Which values become
; workflow trigger attributes is not an acceptance rule and is checked by the
; Go tests.
; trace:formalizes GOJ-REF GOJ-SHA GOJ-ACTOR GOJ-ACTOR-ID GOJ-WORKFLOW-REF GOJ-EVENT-NAME GOJ-ENVIRONMENT GOJ-JOB-WORKFLOW-REF GOJ-TRIGGER-ATTRIBUTES
(define-fun gates-audit-name ((n String)) Bool
  (or (= n "actor") (= n "actor_id") (= n "event_name")
      (= n "ref") (= n "sha") (= n "workflow_ref")
      (= n "job_workflow_ref") (= n "environment")))
; Every other name, repo_property_* and future Claims included.
; trace:formalizes GOJ-DYNAMIC-CLAIM
(define-fun gates-dynamic-name ((n String)) Bool
  (not (or (consulted-name n) (gates-unused-name n) (gates-audit-name n))))
; trace:unformalized GOJ-CLAIMS-JSON モデルは妥当で重複のない JSON をデコードした後の値を扱い、JSON のテキスト、UTF-8、重複した名前を扱わない
; trace:unformalized GOJ-VERIFIED-CLAIMS-SET Claims Set を記録するだけで、受理と拒否を決めない

; Every condition but gates-exp, gates-time and gates-aliases-absent, which
; mutations vary.
(define-fun gates-common-except-exp ((c Claims) (expected String)) Bool
  (and (gates-iss c)
       (gates-aud c expected)
       (gates-iat c)
       (gates-nbf c)
       (gates-sub-format c)
       (gates-sub-length c)
       (gates-id-match c)
       (gates-repository-id c)
       (gates-repository-owner-id c)
       (gates-run-id c)
       (gates-run-attempt c)))
(define-fun gates-claims-accepted ((c Claims) (now Int) (expected String)) Bool
  (and (gates-common-except-exp c expected)
       (gates-exp c)
       (gates-time c now)
       (gates-aliases-absent c)))
