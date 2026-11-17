; GitHub.com: fixed Issuer and configurable audience.
; GitHub Actions OIDC Constraints; https://docs.github.com/en/actions/reference/security/oidc
; Claims_supported and observations do NOT imply all claims are always present,
; nor that all subjects have immutable form. GATES itself selects a subset.
(define-fun github-issuer ((c Claims)) Bool
  (string-is c "iss" "https://token.actions.githubusercontent.com"))
