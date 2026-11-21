; GITHUB, DOCUMENTED -- GitHub Actions OIDC clauses whose Evidence is
; Documented, on the decoded JOSE Header and Claims Set that GitHub issues.
; Written from the clauses alone, without the GATES acceptance. A clause on a
; Claim value constrains the value when the Claim is present, because no
; Documented clause says when a Claim is present (GH-OIDC-CLAIM-PRESENCE is
; Unconfirmed). An Unconfirmed clause is not formalized: leaving it
; unconstrained lets the compatibility checks find the issuance it allows.
; The clauses on Discovery and JWKS are formalized with the signature model.

(define-fun gh-issuer () String "https://token.actions.githubusercontent.com")
; trace:formalizes GH-OIDC-ISS
(define-fun gh-iss ((c Claims)) Bool
  (=> ((_ is Present) (get-member c "iss"))
      (= (get-member c "iss") (Present (JString gh-issuer)))))

; trace:unformalized GH-OIDC-HEADER-DOCUMENTED Header Parameter の意味を説明し、有無と値を定めない

; The names that the OIDC token claims table describes, repo_property_* included.
; trace:formalizes GH-OIDC-CLAIMS-TOKEN-TABLE
(define-fun gh-claims-token-table-name ((n String)) Bool
  (or (= n "aud") (= n "iss") (= n "sub") (= n "exp") (= n "iat") (= n "jti")
      (= n "nbf") (= n "actor") (= n "actor_id") (= n "base_ref")
      (= n "check_run_id") (= n "environment") (= n "event_name")
      (= n "head_ref") (= n "job_workflow_ref") (= n "job_workflow_sha")
      (= n "ref") (= n "ref_type") (= n "repository_visibility")
      (= n "repository") (= n "repository_id") (= n "repository_owner")
      (= n "repository_owner_id") (str.prefixof "repo_property_" n)
      (= n "run_id") (= n "run_number") (= n "run_attempt")
      (= n "runner_environment") (= n "workflow") (= n "workflow_ref")
      (= n "workflow_sha")))

; The JWT is used at usedAt within the meaning of its time Claims: before
; exp, at or after nbf, and at or after the time iat says it was issued. jti
; identifies one token, which a model of one Claims Set does not constrain.
; trace:formalizes GH-OIDC-REGISTERED-CLAIMS
(define-fun gh-registered-claims ((c Claims) (usedAt Int)) Bool
  (let ((e (get-member c "exp")) (i (get-member c "iat")) (b (get-member c "nbf")))
    (and (=> (and ((_ is Present) e) (json-number (member-value e)))
             (< (to_real usedAt) (json-number-seconds (member-value e))))
         (=> (and ((_ is Present) i) (json-number (member-value i)))
             (<= (json-number-seconds (member-value i)) (to_real usedAt)))
         (=> (and ((_ is Present) b) (json-number (member-value b)))
             (<= (json-number-seconds (member-value b)) (to_real usedAt))))))

; repositoryId and ownerId are the IDs of the repository and its owner. The
; clauses do not give the JSON type, so they constrain only a string value.
; trace:formalizes GH-OIDC-REPOSITORY-ID
(define-fun gh-repository-id ((c Claims) (repositoryId String)) Bool
  (=> (string-member c "repository_id")
      (= (string-member-value c "repository_id") repositoryId)))
; trace:formalizes GH-OIDC-REPOSITORY-OWNER-ID
(define-fun gh-repository-owner-id ((c Claims) (ownerId String)) Bool
  (=> (string-member c "repository_owner_id")
      (= (string-member-value c "repository_owner_id") ownerId)))

; trace:unformalized GH-OIDC-RUN-ID Claim の意味だけを定め、値の型と形式を定めない
; trace:unformalized GH-OIDC-RUN-ATTEMPT Claim の意味だけを定め、値の型と形式を定めない
; trace:unformalized GH-OIDC-ACTOR Claim の意味だけを定め、値の型と形式を定めない
; trace:unformalized GH-OIDC-EVENT-NAME Claim の意味だけを定め、値の型と形式を定めない
; trace:unformalized GH-OIDC-REF Claim の意味だけを定め、値の型と形式を定めない
; trace:unformalized GH-OIDC-WORKFLOW-REF Claim の意味だけを定め、値の型と形式を定めない
; trace:unformalized GH-OIDC-ENVIRONMENT Claim の意味だけを定め、値の型と形式を定めない
; trace:unformalized GH-OIDC-REPOSITORY-ID-RENAME 改名の前と後に発行された二つの Claims Set の関係で、一つの Claims Set の値の制約ではない
; dependabot tells that a Dependabot update job requested the JWT.
; trace:formalizes GH-OIDC-EVENT-NAME-DEPENDABOT
(define-fun gh-event-name-dependabot ((c Claims) (dependabot Bool)) Bool
  (=> dependabot (= (get-member c "event_name") (Present (JString "dynamic")))))

; trace:formalizes GH-OIDC-REPO-PROPERTY-VALUE
(define-fun gh-repo-property-value ((c Claims) (n String)) Bool
  (=> (and (str.prefixof "repo_property_" n) ((_ is Present) (get-member c n)))
      (string-member c n)))
; configured tells that an administrator added the property to the OIDC
; settings, and valueSet that the repository has a value for it.
; trace:formalizes GH-OIDC-REPO-PROPERTY-PRESENCE
(define-fun gh-repo-property-presence ((c Claims) (n String) (configured Bool) (valueSet Bool)) Bool
  (=> (and (str.prefixof "repo_property_" n) configured valueSet)
      ((_ is Present) (get-member c n))))

; The value of aud is a value given as the Audience: as a string, or as the
; one-element array that RFC 7519 lets a single Audience take. ownerUrl is the
; URL of the repository owner, and requested the Audience the job specifies.
(define-fun aud-is ((c Claims) (value String)) Bool
  (=> ((_ is Present) (get-member c "aud"))
      (or (= (get-member c "aud") (Present (JString value)))
          (= (get-member c "aud") (Present (JStringArray (seq.unit value)))))))
; trace:formalizes GH-OIDC-AUD-DEFAULT
(define-fun gh-aud-default ((c Claims) (ownerUrl String)) Bool
  (aud-is c ownerUrl))
; trace:formalizes GH-OIDC-AUD-CUSTOM
(define-fun gh-aud-custom ((c Claims) (requested String)) Bool
  (aud-is c requested))

; The legacy default sub has the names alone, in the syntax that the source of
; GH-OIDC-SUB-IMMUTABLE-FORMAT shows: repo:OWNER/REPO:CONTEXT.
; trace:formalizes GH-OIDC-SUB-IMMUTABLE-REASON
(define-fun gh-sub-legacy ((c Claims) (owner String) (repo String) (context String)) Bool
  (=> ((_ is Present) (get-member c "sub"))
      (= (get-member c "sub")
         (Present (JString (str.++ "repo:" owner "/" repo ":" context))))))
; trace:formalizes GH-OIDC-SUB-IMMUTABLE-FORMAT
(define-fun gh-sub-immutable ((c Claims) (owner String) (ownerId String) (repo String) (repositoryId String) (context String)) Bool
  (=> ((_ is Present) (get-member c "sub"))
      (= (get-member c "sub")
         (Present (JString (str.++ "repo:" owner "@" ownerId "/" repo "@" repositoryId ":" context))))))
; trace:formalizes GH-OIDC-SUB-AT-SEPARATOR
(define-fun gh-name-without-at ((name String)) Bool
  (not (str.contains name "@")))
; trace:formalizes GH-OIDC-SUB-CONTEXT
(define-fun gh-sub-context ((context String)) Bool
  (or (= context "pull_request")
      (and (str.prefixof "environment:" context)
           (not (str.contains (str.substr context 12 (- (str.len context) 12)) ":")))
      (and (str.prefixof "ref:" context)
           (not (str.contains (str.substr context 4 (- (str.len context) 4)) ":")))))

; The form of the default sub of a repository. createdAfter tells that the
; repository was created after the date the clauses name, optedIn that the
; organization or the repository opted in to the immutable form, renamed that
; it was renamed or transferred after that date, and customized that a
; template applies to it.
(declare-datatypes () ((SubjectForm ImmutableForm LegacyForm CustomizedForm)))
; trace:formalizes GH-OIDC-SUB-IMMUTABLE-DEFAULT
(define-fun gh-sub-immutable-default ((createdAfter Bool) (customized Bool) (form SubjectForm)) Bool
  (=> (and createdAfter (not customized)) (= form ImmutableForm)))
; trace:formalizes GH-OIDC-SUB-LEGACY-RETAINED
(define-fun gh-sub-legacy-retained ((createdAfter Bool) (optedIn Bool) (renamed Bool) (customized Bool) (form SubjectForm)) Bool
  (=> (and (not createdAfter) (not optedIn) (not renamed) (not customized))
      (= form LegacyForm)))
; trace:formalizes GH-OIDC-SUB-RENAME-TRANSFER
(define-fun gh-sub-rename-transfer ((renamed Bool) (customized Bool) (form SubjectForm)) Bool
  (=> (and renamed (not customized)) (= form ImmutableForm)))
; The template replaces the whole default form; what it yields is Unconfirmed
; (GH-OIDC-SUB-CUSTOM-WITHOUT-REPO, GH-OIDC-SUB-CUSTOM-REPO-ONLY).
; trace:formalizes GH-OIDC-SUB-CUSTOM-REPLACE
(define-fun gh-sub-custom-replace ((customized Bool) (form SubjectForm)) Bool
  (=> customized (= form CustomizedForm)))
; immutableRepository tells that the repository uses the immutable form, and
; withRepo that the template includes repo.
; trace:formalizes GH-OIDC-SUB-CUSTOM-IMMUTABLE-IDS
(define-fun gh-sub-custom-immutable-ids ((c Claims) (form SubjectForm) (immutableRepository Bool) (withRepo Bool) (owner String) (ownerId String) (repo String) (repositoryId String)) Bool
  (=> (and (= form CustomizedForm) immutableRepository withRepo (string-member c "sub"))
      (str.contains (string-member-value c "sub")
                    (str.++ "repo:" owner "@" ownerId "/" repo "@" repositoryId))))
; trace:unformalized GH-OIDC-SUB-CUSTOM-EXAMPLES テンプレートの例で、発行される sub の値の制約を定めない
; trace:unformalized GH-OIDC-SUB-ORG-TEMPLATE-OPT-IN テンプレートを適用する設定の定めで、発行される Claims Set の値の制約ではない
; trace:unformalized GH-OIDC-SUB-ORG-TEMPLATE-SEPARATE REST API のパラメーターの定めで、発行される Claims Set の値の制約ではない

; The default sub of a repository in its form, with the names and IDs of the
; owner and the repository. A customized sub is not constrained here.
(define-fun gh-sub-form ((c Claims) (form SubjectForm) (owner String) (ownerId String) (repo String) (repositoryId String) (context String)) Bool
  (and (=> (= form ImmutableForm) (gh-sub-immutable c owner ownerId repo repositoryId context))
       (=> (= form LegacyForm) (gh-sub-legacy c owner repo context))
       (=> (not (= form CustomizedForm)) (gh-sub-context context))))

; The official example JWT. Its run_id, jti, sha, workflow, x5t and kid are
; placeholders that show only the JSON type.
; trace:formalizes GH-OIDC-CLAIM-TYPE-EXAMPLE GH-OIDC-TIME-EXAMPLE GH-OIDC-SUB-EXAMPLE-LEGACY
(define-fun gh-example-claims () Claims
  (store (store (store (store (store (store (store (store (store (store
  (store (store (store (store (store (store (store (store (store (store
  (store (store (store (store (store (store (store (store
    ((as const Claims) Absent)
    "jti" (Present (JString "example-id")))
    "sub" (Present (JString "repo:octo-org/octo-repo:environment:prod")))
    "environment" (Present (JString "prod")))
    "aud" (Present (JString "https://github.com/octo-org")))
    "ref" (Present (JString "refs/heads/main")))
    "sha" (Present (JString "example-sha")))
    "repository" (Present (JString "octo-org/octo-repo")))
    "repository_owner" (Present (JString "octo-org")))
    "actor_id" (Present (JString "12")))
    "repository_visibility" (Present (JString "private")))
    "repository_id" (Present (JString "74")))
    "repository_owner_id" (Present (JString "65")))
    "run_id" (Present (JString "example-run-id")))
    "run_number" (Present (JString "10")))
    "run_attempt" (Present (JString "2")))
    "actor" (Present (JString "octocat")))
    "workflow" (Present (JString "example-workflow")))
    "head_ref" (Present (JString "")))
    "base_ref" (Present (JString "")))
    "event_name" (Present (JString "workflow_dispatch")))
    "ref_type" (Present (JString "branch")))
    "job_workflow_ref" (Present (JString "octo-org/octo-automation/.github/workflows/oidc.yml@refs/heads/main")))
    "iss" (Present (JString "https://token.actions.githubusercontent.com")))
    "nbf" (Present (JInteger 1632492967)))
    "exp" (Present (JInteger 1632493867)))
    "iat" (Present (JInteger 1632493567)))
    "runner_environment" (Present (JString "github-hosted")))
    "repo_property_workspace_id" (Present (JString "ws-abc123"))))
; trace:formalizes GH-OIDC-HEADER-EXAMPLE
(define-fun gh-example-header () Header
  (store (store (store (store ((as const Header) Absent)
    "typ" (Present (JString "JWT")))
    "alg" (Present (JString "RS256")))
    "x5t" (Present (JString "example-thumbprint")))
    "kid" (Present (JString "example-key-id"))))
