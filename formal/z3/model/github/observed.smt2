; GITHUB, OBSERVED -- GitHub Actions OIDC clauses whose Evidence is Published,
; Observed or Inferred, on the decoded JOSE Header and Claims Set that GitHub
; issues. The compatibility checks add them to model/github/documented.smt2 as
; the second stage. Published values are recorded at one time and Observed
; values under the recorded conditions; neither is a guarantee by GitHub, so
; both belong to this stage, and an Inferred clause is in the stage of the
; clauses it is drawn from. Every Observed clause records JWTs of private
; repositories owned by a personal account (GH-OIDC-ORGANIZATION-JWT).

; The Discovery advertises RS256 as the only signing algorithm, read here as
; the alg of a JWT that GitHub signs.
; trace:formalizes GH-OIDC-DISCOVERY-ALG
(define-fun gh-discovery-alg ((h Header)) Bool
  (=> ((_ is Present) (get-member h "alg"))
      (= (get-member h "alg") (Present (JString "RS256")))))
; trace:formalizes GH-OIDC-DISCOVERY-CLAIMS
(define-fun gh-discovery-claims-name ((n String)) Bool
  (or (= n "sub") (= n "aud") (= n "exp") (= n "iat") (= n "iss") (= n "jti")
      (= n "nbf") (= n "ref") (= n "sha") (= n "repository")
      (= n "repository_id") (= n "repository_owner")
      (= n "repository_owner_id") (= n "enterprise") (= n "enterprise_id")
      (= n "run_id") (= n "run_number") (= n "run_attempt") (= n "actor")
      (= n "actor_id") (= n "workflow") (= n "workflow_ref")
      (= n "workflow_sha") (= n "head_ref") (= n "base_ref")
      (= n "event_name") (= n "ref_type") (= n "ref_protected")
      (= n "environment") (= n "environment_node_id")
      (= n "job_workflow_ref") (= n "job_workflow_sha")
      (= n "repository_visibility") (= n "runner_environment")
      (= n "issuer_scope") (= n "check_run_id")))
; trace:formalizes GH-OIDC-CLAIMS-UNDESCRIBED
(define-fun gh-claims-undescribed-name ((n String)) Bool
  (and (gh-discovery-claims-name n) (not (gh-claims-token-table-name n))))
; trace:formalizes GH-OIDC-JWKS-KID-FORMAT
(define-fun gh-jwks-kid-format ((kid String)) Bool
  (let ((hex (re.union (re.range "0" "9") (re.range "a" "f") (re.range "A" "F"))))
    (or (str.in_re kid (re.++ ((_ re.loop 8 8) hex) (str.to_re "-")
                              ((_ re.loop 4 4) hex) (str.to_re "-")
                              ((_ re.loop 4 4) hex) (str.to_re "-")
                              ((_ re.loop 4 4) hex) (str.to_re "-")
                              ((_ re.loop 12 12) hex)))
        (str.in_re kid ((_ re.loop 40 40) hex)))))

; The JOSE Header has alg RS256, kid, typ and x5t, and nothing else.
; trace:formalizes GH-OIDC-HEADER-OBSERVED
(define-fun gh-header-observed ((h Header) (kid JsonValue) (typ JsonValue) (x5t JsonValue)) Bool
  (= h (store (store (store (store ((as const Header) Absent)
         "alg" (Present (JString "RS256")))
         "kid" (Present kid))
         "typ" (Present typ))
         "x5t" (Present x5t))))
; jwksKid is the kid of the one JWKS key whose kid and x5t the Header matches.
; trace:formalizes GH-OIDC-HEADER-KID-JWKS
(define-fun gh-header-kid-jwks ((h Header) (jwksKid String)) Bool
  (= (get-member h "kid") (Present (JString jwksKid))))

(define-fun digits-text ((s String)) Bool
  (str.in_re s (re.+ (re.range "0" "9"))))
(define-fun digits-or-absent ((c Claims) (n String)) Bool
  (or (= (get-member c n) Absent)
      (and (string-member c n) (digits-text (string-member-value c n)))))
; The names whose values the clause records as strings, other than the ones
; recorded as numbers or as strings of digits.
(define-fun observed-string-name ((n String)) Bool
  (or (= n "sub") (= n "aud") (= n "iss") (= n "jti") (= n "ref") (= n "sha")
      (= n "repository") (= n "repository_owner") (= n "actor")
      (= n "workflow") (= n "workflow_ref") (= n "workflow_sha")
      (= n "head_ref") (= n "base_ref") (= n "event_name") (= n "ref_type")
      (= n "ref_protected") (= n "environment") (= n "environment_node_id")
      (= n "job_workflow_ref") (= n "job_workflow_sha")
      (= n "repository_visibility") (= n "runner_environment")))
; trace:formalizes GH-OIDC-CLAIM-TYPE-OBSERVED
(define-fun gh-claim-type-observed ((c Claims) (n String)) Bool
  (and (=> (and (or (= n "exp") (= n "iat") (= n "nbf")) ((_ is Present) (get-member c n)))
           (json-number (member-value (get-member c n))))
       (=> (or (= n "repository_id") (= n "repository_owner_id") (= n "actor_id")
               (= n "run_id") (= n "run_attempt") (= n "run_number")
               (= n "check_run_id"))
           (digits-or-absent c n))
       (=> (observed-string-name n) (string-or-absent c n))))
; usesEnvironment tells that the job uses an Environment, and pullRequest that
; a pull_request event started the workflow.
; trace:formalizes GH-OIDC-CLAIM-PRESENCE-OBSERVED
(define-fun gh-claim-presence-observed ((c Claims) (usesEnvironment Bool) (pullRequest Bool)) Bool
  (and (= usesEnvironment ((_ is Present) (get-member c "environment")))
       (= usesEnvironment ((_ is Present) (get-member c "environment_node_id")))
       (=> (not pullRequest)
           (and (= (get-member c "base_ref") (Present (JString "")))
                (= (get-member c "head_ref") (Present (JString "")))))
       (= (get-member c "enterprise") Absent)
       (= (get-member c "enterprise_id") Absent)
       (= (get-member c "issuer_scope") Absent)))
; trace:formalizes GH-OIDC-REPOSITORY-OWNER-ID-OBSERVED
(define-fun gh-repository-owner-id-observed ((c Claims)) Bool
  (and ((_ is Present) (get-member c "repository_owner_id"))
       (= (get-member c "repository_owner_id") (get-member c "actor_id"))))
; trace:unformalized GH-OIDC-REPOSITORY-ID-REST-OBSERVED REST API の応答の値との関係で、一つの Claims Set の値の制約ではない
; trace:unformalized GH-OIDC-REPOSITORY-NAME-OBSERVED REST API の応答の値と、改名の前と後の Claims Set との関係で、一つの Claims Set の値の制約ではない
; trace:formalizes GH-OIDC-RUN-ATTEMPT-OBSERVED
(define-fun gh-run-attempt-observed ((c Claims)) Bool
  (= (get-member c "run_attempt") (Present (JString "1"))))
; reusable tells that the job uses a reusable workflow.
; trace:formalizes GH-OIDC-WORKFLOW-REF-OBSERVED
(define-fun gh-workflow-ref-observed ((c Claims) (reusable Bool)) Bool
  (and ((_ is Present) (get-member c "job_workflow_ref"))
       (=> (not reusable)
           (= (get-member c "job_workflow_ref") (get-member c "workflow_ref")))))
; trace:formalizes GH-OIDC-TIME-OBSERVED
(define-fun gh-time-observed ((c Claims)) Bool
  (let ((e (get-member c "exp")) (i (get-member c "iat")) (b (get-member c "nbf")))
    (and ((_ is Present) e) ((_ is Present) i) ((_ is Present) b)
         (json-number (member-value e)) (json-number (member-value i))
         (json-number (member-value b))
         (= (json-number-seconds (member-value b)) (- (json-number-seconds (member-value i)) 300.0))
         (= (json-number-seconds (member-value e)) (+ (json-number-seconds (member-value i)) 300.0)))))
; trace:formalizes GH-OIDC-AUD-DEFAULT-OBSERVED
(define-fun gh-aud-default-observed ((c Claims) (ownerUrl String)) Bool
  (= (get-member c "aud") (Present (JString ownerUrl))))
; The records are of Audiences that are not URLs, read here as values without
; "://"; the clause says nothing of a URL.
; trace:formalizes GH-OIDC-AUD-CUSTOM-STRING
(define-fun gh-aud-custom-string ((c Claims) (requested String)) Bool
  (=> (not (str.contains requested "://"))
      (= (get-member c "aud") (Present (JString requested)))))
; trace:unformalized GH-OIDC-AUD-CUSTOM-ISSUANCE Audience の値ごとに JWT が発行されたかの記録で、発行された Claims Set の値の制約ではない
; The sub has the immutable form, with one of the recorded contexts.
; trace:formalizes GH-OIDC-SUB-OBSERVED
(define-fun gh-sub-observed ((c Claims) (owner String) (ownerId String) (repo String) (repositoryId String) (context String)) Bool
  (and (gh-sub-immutable c owner ownerId repo repositoryId context)
       ((_ is Present) (get-member c "sub"))
       (or (= context "pull_request")
           (= context "environment:testing")
           (str.prefixof "ref:refs/heads/" context))))
; trace:unformalized GH-OIDC-SUB-FORMAT-PER-REPO Organization の設定と個々のリポジトリの sub の形式の関係で、一つの Claims Set の値の制約ではない
