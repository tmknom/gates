; Compact accepted decoded Claims Set and JOSE Header, with all other names
; absent. now=1000, expected audience="gates-oidc". NOT a real GitHub-issued JWT.
(define-fun sample () Claims
  (store (store (store (store (store (store (store (store (store
    ((as const Claims) Absent)
    "iss" (Present (JString "https://token.actions.githubusercontent.com")))
    "aud" (Present (JString "gates-oidc")))
    "exp" (Present (JInteger 1001)))
    "iat" (Present (JInteger 1000)))
    "sub" (Present (JString "repo:org@1/repo@2:ctx")))
    "repository_owner_id" (Present (JString "1")))
    "repository_id" (Present (JString "2")))
    "run_id" (Present (JString "3")))
    "run_attempt" (Present (JString "1"))))
(define-fun sample-header () Header
  (store (store ((as const Header) Absent)
    "alg" (Present (JString "RS256")))
    "kid" (Present (JString "k1"))))
; Acceptance of sample with one Claim replaced, at now=1000 for "gates-oidc".
(define-fun accepts-with ((name String) (value Member)) Bool
  (gates-claims-accepted (store sample name value) 1000 "gates-oidc"))
; Acceptance of sample-header with one Header Parameter replaced.
(define-fun header-accepts-with ((name String) (value Member)) Bool
  (gates-header-accepted (store sample-header name value)))
; A name equal, under the string comparison of the clauses, to one of the names
; the rules read, or an alias, which GATES reads in order to reject it.
(define-fun same-name ((n String) (m String)) Bool
  (and (std-oidc-string-equal n m) (std-jwt-string-equal n m)))
(define-fun read-name ((n String)) Bool
  (or (same-name n "iss") (same-name n "aud") (same-name n "exp")
      (same-name n "iat") (same-name n "nbf") (same-name n "sub")
      (same-name n "repository_owner_id") (same-name n "repository_id")
      (same-name n "run_id") (same-name n "run_attempt")
      (claim-alias n)))

; COMPATIBILITY -- the GitHub issuance that checks/compat/ gives to the GATES
; acceptance. issued and issuedHeader are a JWT that GitHub issues to a job
; that requests the Audience expected, and GATES verifies it at now on the
; Lambda clock. usedAt is the GitHub clock when the job uses it. The form and
; the repository state choose the default sub; the names and IDs are those of
; the owner and the repository.
(declare-const issued Claims)
(declare-const issuedHeader Header)
(declare-const now Int)
(declare-const usedAt Int)
(declare-const expected String)
(declare-const form SubjectForm)
(declare-const createdAfter Bool)
(declare-const optedIn Bool)
(declare-const renamed Bool)
(declare-const customized Bool)
(declare-const owner String)
(declare-const ownerId String)
(declare-const repo String)
(declare-const repositoryId String)
(declare-const context String)
(declare-const kid String)
(declare-const typ JsonValue)
(declare-const x5t JsonValue)
(declare-const usesEnvironment Bool)
(declare-const pullRequest Bool)
(declare-const reusable Bool)
; The issuance that every Documented clause allows. The Header has no
; Documented constraint (GH-OIDC-HEADER-DOCUMENTED describes the parameters
; without their values), and an Unconfirmed clause constrains nothing.
(define-fun documented-issuance () Bool
  (and (gh-iss issued)
       (gh-registered-claims issued usedAt)
       (gh-repository-id issued repositoryId)
       (gh-repository-owner-id issued ownerId)
       (gh-aud-custom issued expected)
       (gh-name-without-at owner) (gh-name-without-at repo)
       (gh-sub-immutable-default createdAfter customized form)
       (gh-sub-legacy-retained createdAfter optedIn renamed customized form)
       (gh-sub-rename-transfer renamed customized form)
       (gh-sub-custom-replace customized form)
       (gh-sub-form issued form owner ownerId repo repositoryId context)))
; The issuance that every Published and Observed clause allows as well. Most
; Observed clauses record JWTs issued without an Audience requested; applying
; them to the JWT issued for expected is a premise of the checks, not a record.
(define-fun observed-issuance () Bool
  (and documented-issuance
       (gh-discovery-alg issuedHeader)
       (gh-header-observed issuedHeader (JString kid) typ x5t)
       (gh-header-kid-jwks issuedHeader kid)
       (gh-jwks-kid-format kid)
       (gh-claim-type-observed issued "exp")
       (gh-claim-type-observed issued "iat")
       (gh-claim-type-observed issued "nbf")
       (gh-claim-type-observed issued "repository_id")
       (gh-claim-type-observed issued "repository_owner_id")
       (gh-claim-type-observed issued "run_id")
       (gh-claim-type-observed issued "run_attempt")
       (gh-claim-presence-observed issued usesEnvironment pullRequest)
       (gh-repository-owner-id-observed issued)
       (gh-run-attempt-observed issued)
       (gh-workflow-ref-observed issued reusable)
       (gh-time-observed issued)
       (gh-aud-custom-string issued expected)
       (gh-sub-observed issued owner ownerId repo repositoryId context)))
; The default sub has the immutable form: the repository was created after the
; date, and no template applies. This excludes the issuance of
; GH-OIDC-SUB-LEGACY-RETAINED and GH-OIDC-SUB-CUSTOM-REPLACE.
(define-fun immutable-repository () Bool
  (and createdAfter (not customized)))
; Both clocks are within the range of GOJ-EXP, with room for the 300 seconds
; between nbf, iat and exp that GH-OIDC-TIME-OBSERVED records.
(define-fun clocks-in-range () Bool
  (and (<= 600 usedAt) (<= (+ usedAt 300) 253402300799)))
; The GitHub clock is at most L ahead of the Lambda clock.
(define-fun clock-skew-within-leeway () Bool
  (<= usedAt (+ now gates-leeway)))
; A Claim GATES does not consult replaced with m, and the rest of c kept.
(define-fun with-member ((c Claims) (name String) (m Member)) Claims
  (lambda ((n String)) (ite (and (= n name) (not (consulted-name n))) m (select c n))))
(define-fun header-with-member ((h Header) (name String) (m Member)) Header
  (lambda ((n String)) (ite (and (= n name) (not (header-consulted-name n))) m (select h n))))
; Acceptance of issued differs between a Claim m under name and none.
(define-fun claim-changes-acceptance ((name String) (m Member)) Bool
  (not (= (gates-claims-accepted (with-member issued name m) now expected)
          (gates-claims-accepted (with-member issued name Absent) now expected))))
; The names and IDs of the immutable sub in GH-OIDC-SUB-IMMUTABLE-FORMAT. Z3
; answers unknown on where str.indexof finds '/' and ':' in a sub built from
; arbitrary names and IDs, so the checks on the sub parser fix them and leave
; the rest of the issuance arbitrary. The fixed names have neither '/' in the
; owner name nor ':' in the repository name, which excludes the issuance of
; the Possibly Rejects row of GOJ-SUB-FORMAT on GH-OIDC-SUB-LENGTH, and the
; fixed IDs have the GOJ-ID-FORMAT spelling, which excludes the issuance of
; GH-OIDC-CLAIM-ID-FORMAT.
(define-fun documented-names () Bool
  (and (= owner "octo-org") (= ownerId "123456")
       (= repo "octo-repo") (= repositoryId "456789")))
; The names and IDs of the recorded JWTs.
(define-fun observed-names () Bool
  (and (= owner "octocat") (= ownerId "583231")
       (= repo "example-repo") (= repositoryId "1234567890")))
