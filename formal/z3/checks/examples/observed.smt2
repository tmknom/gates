; expect: sat sat sat sat sat sat sat
; trace:checks GH-OIDC-ISS GH-OIDC-REGISTERED-CLAIMS GH-OIDC-REPOSITORY-ID GH-OIDC-REPOSITORY-OWNER-ID GH-OIDC-AUD-DEFAULT GH-OIDC-SUB-AT-SEPARATOR GH-OIDC-SUB-CONTEXT GH-OIDC-SUB-IMMUTABLE-FORMAT GH-OIDC-CLAIM-TYPE-OBSERVED GH-OIDC-CLAIM-PRESENCE-OBSERVED GH-OIDC-REPOSITORY-OWNER-ID-OBSERVED GH-OIDC-RUN-ATTEMPT-OBSERVED GH-OIDC-WORKFLOW-REF-OBSERVED GH-OIDC-TIME-OBSERVED GH-OIDC-AUD-DEFAULT-OBSERVED GH-OIDC-SUB-OBSERVED GH-OIDC-DISCOVERY-ALG GH-OIDC-HEADER-OBSERVED GH-OIDC-HEADER-KID-JWKS GH-OIDC-JWKS-KID-FORMAT RFC7519-ISS-FORMAT RFC7519-SUB-FORMAT RFC7519-JTI-FORMAT RFC7519-EXP-NUMERICDATE RFC7519-IAT-NUMERICDATE RFC7519-NBF-NUMERICDATE RFC7519-AUD-INTENDED RFC7519-EXP-REJECT RFC7519-NBF-REJECT RFC7519-TYP-VALUE RFC7515-ALG-REQUIRED RFC7515-ALG-FORMAT RFC7515-KID-FORMAT RFC7515-KEY-IDENTIFY RFC7518-NONE-ACCEPT-DEFAULT RFC7797-JWT-B64-FALSE
; The six JWTs that GitHub issued (docs/research/github/oidc-jwt.md), put
; into all three layers at one second after iat. Each satisfies the clauses
; of the standards on what the Issuer puts in the JWT, the GitHub clauses
; with the conditions it was recorded under, and the GATES acceptance with
; the default Audience as the expected one. A JWT that failed the standards
; would show a misreading of a clause or GitHub against the standard. The
; records are of private repositories that a personal account owns, fetched
; without an Audience; they do not show that GitHub always issues this form.
(define-fun recorded ((jti String) (exp Int) (iat Int) (nbf Int) (context String) (run String) (event String) (environment Member) (baseRef String) (headRef String) (workflowRef String) (jobWorkflowRef String)) Claims
  (store (store (store (store (store (store (store (store (store (store
  (store (store (store (store (store (store (store (store (store (store (store
    ((as const Claims) Absent)
    "iss" (Present (JString "https://token.actions.githubusercontent.com")))
    "aud" (Present (JString "https://github.com/octocat")))
    "exp" (Present (JInteger exp)))
    "iat" (Present (JInteger iat)))
    "nbf" (Present (JInteger nbf)))
    "sub" (Present (JString (str.++ "repo:octocat@583231/example-repo@1234567890:" context))))
    "repository_owner_id" (Present (JString "583231")))
    "repository_id" (Present (JString "1234567890")))
    "run_id" (Present (JString run)))
    "run_attempt" (Present (JString "1")))
    "actor" (Present (JString "octocat")))
    "actor_id" (Present (JString "583231")))
    "event_name" (Present (JString event)))
    "jti" (Present (JString jti)))
    "ref_protected" (Present (JString "false")))
    "base_ref" (Present (JString baseRef)))
    "head_ref" (Present (JString headRef)))
    "environment" environment)
    "environment_node_id" (ite ((_ is Present) environment) (Present (JString "EN_kwDOSZYC0s8AAAABKgfUQA")) Absent))
    "workflow_ref" (Present (JString workflowRef)))
    "job_workflow_ref" (Present (JString jobWorkflowRef))))
(define-fun recorded-header () Header
  (store (store (store (store ((as const Header) Absent)
    "alg" (Present (JString "RS256")))
    "kid" (Present (JString "38826b17-6a30-5f9b-b169-8beb8202f723")))
    "typ" (Present (JString "JWT")))
    "x5t" (Present (JString "ykNaY4qM_ta4k2TgZOCEYLkcYlA"))))
(define-fun standards ((c Claims) (now Int)) Bool
  (and (std-iss-format c) (std-sub-format c) (std-jti-format c)
       (std-exp-numericdate c) (std-iat-numericdate c) (std-nbf-numericdate c)
       (std-aud-identifies c "https://github.com/octocat")
       (std-exp-reject c now 0.0) (std-nbf-reject c now 0.0)))
(define-fun github ((c Claims) (now Int) (context String) (usesEnvironment Bool) (pullRequest Bool) (reusable Bool)) Bool
  (and (gh-iss c) (gh-registered-claims c now)
       (gh-repository-id c "1234567890") (gh-repository-owner-id c "583231")
       (gh-aud-default c "https://github.com/octocat")
       (gh-name-without-at "octocat") (gh-name-without-at "example-repo")
       (gh-sub-form c ImmutableForm "octocat" "583231" "example-repo" "1234567890" context)
       (gh-claim-type-observed c "exp") (gh-claim-type-observed c "iat")
       (gh-claim-type-observed c "nbf") (gh-claim-type-observed c "repository_id")
       (gh-claim-type-observed c "run_id") (gh-claim-type-observed c "ref_protected")
       (gh-claim-presence-observed c usesEnvironment pullRequest)
       (gh-repository-owner-id-observed c) (gh-run-attempt-observed c)
       (gh-workflow-ref-observed c reusable) (gh-time-observed c)
       (gh-aud-default-observed c "https://github.com/octocat")
       (gh-sub-observed c "octocat" "583231" "example-repo" "1234567890" context)))
(define-fun all-layers ((c Claims) (iat Int) (context String) (pullRequest Bool) (reusable Bool)) Bool
  (and (standards c (+ iat 1))
       (github c (+ iat 1) context ((_ is Present) (select c "environment")) pullRequest reusable)
       (gates-claims-accepted c (+ iat 1) "https://github.com/octocat")))
(define-fun oidc-yml () String "octocat/example-repo/.github/workflows/oidc.yml@")
(define-fun reusable-yml () String "octocat/example-repo/.github/workflows/reusable.yml@")
(push 1) (assert (all-layers (recorded "dec56e62-02aa-482d-a954-d0aaaef107fc" 1793340472 1793340172 1793339872 "ref:refs/heads/test-branch" "41011783574" "push" Absent "" "" (str.++ oidc-yml "refs/heads/test-branch") (str.++ oidc-yml "refs/heads/test-branch")) 1793340172 "ref:refs/heads/test-branch" false false)) (check-sat) (pop 1) ; push: sat
(push 1) (assert (all-layers (recorded "37ef094d-44fa-46f0-ad9f-ea55842787a2" 1793340937 1793340637 1793340337 "pull_request" "41012535428" "pull_request" Absent "main" "test-branch" (str.++ oidc-yml "refs/pull/1/merge") (str.++ oidc-yml "refs/pull/1/merge")) 1793340637 "pull_request" true false)) (check-sat) (pop 1) ; pull_request: sat
(push 1) (assert (all-layers (recorded "7c5220f0-fde0-4d9d-998f-7c3c6ea07a88" 1793422055 1793421755 1793421455 "ref:refs/heads/main" "41169659213" "workflow_dispatch" Absent "" "" (str.++ oidc-yml "refs/heads/main") (str.++ oidc-yml "refs/heads/main")) 1793421755 "ref:refs/heads/main" false false)) (check-sat) (pop 1) ; workflow_dispatch: sat
(push 1) (assert (all-layers (recorded "7b5d75a7-98a0-4007-99e6-b6776981bd92" 1793344300 1793344000 1793343700 "ref:refs/heads/test-branch" "41018050776" "push" Absent "" "" (str.++ oidc-yml "refs/heads/test-branch") (str.++ reusable-yml "557ca50fa5a17294d2950cb3992f14dc4d03090e")) 1793344000 "ref:refs/heads/test-branch" false true)) (check-sat) (pop 1) ; absolute reusable workflow: sat
(push 1) (assert (all-layers (recorded "ca9c4c4b-2a70-4bdc-a031-5ec7843a7fdb" 1793343920 1793343620 1793343320 "pull_request" "41017362820" "pull_request" Absent "main" "test-branch" (str.++ oidc-yml "refs/pull/2/merge") (str.++ reusable-yml "refs/pull/2/merge")) 1793343620 "pull_request" true true)) (check-sat) (pop 1) ; relative reusable workflow: sat
(push 1) (assert (all-layers (recorded "6a56642a-879a-4777-b024-4daed3bf14ee" 1793343910 1793343610 1793343310 "environment:testing" "41017362820" "pull_request" (Present (JString "testing")) "main" "test-branch" (str.++ oidc-yml "refs/pull/2/merge") (str.++ oidc-yml "refs/pull/2/merge")) 1793343610 "environment:testing" true false)) (check-sat) (pop 1) ; environment: sat

(push 1)
(assert (and (std-jwt-typ-value recorded-header) (std-jws-alg-present recorded-header)
             (std-jws-alg-format recorded-header) (std-jws-kid-format recorded-header)
             (std-jws-key-identify recorded-header false)
             (std-none-accept-default recorded-header false) (std-jwt-b64-not-false recorded-header)
             (gh-discovery-alg recorded-header)
             (gh-header-observed recorded-header (JString "38826b17-6a30-5f9b-b169-8beb8202f723") (JString "JWT") (JString "ykNaY4qM_ta4k2TgZOCEYLkcYlA"))
             (gh-header-kid-jwks recorded-header "38826b17-6a30-5f9b-b169-8beb8202f723")
             (gh-jwks-kid-format "38826b17-6a30-5f9b-b169-8beb8202f723")
             (gates-header-accepted recorded-header)))
(check-sat) ; Header: sat
(pop 1)
