# GATES

**GitHub App Token Exchange System**

## Description

GATES is a system for issuing GitHub App installation access tokens to GitHub Actions workflows.

It uses GitHub OIDC, AWS Lambda, and AWS KMS to issue short-lived, repository-scoped access tokens without exposing GitHub App private keys to GitHub Actions runners or AWS Lambda.

GATES provides the following capabilities:

- **Keyless authentication**: Authenticate GitHub Actions workflows using OIDC without long-lived AWS credentials.
- **Private key protection**: Store GitHub App private key material in AWS KMS and delegate cryptographic signing operations to KMS.
- **Least-privilege tokens**: Issue installation access tokens scoped to explicitly requested repositories with `contents:read` permission.
- **Self-service access**: Allow GitHub Actions workflows to request tokens through a reusable Composite Action.
- **Infrastructure as Code**: Manage AWS infrastructure using Terraform.

## Architecture

GATES consists of a GitHub Actions Composite Action, AWS STS, AWS Lambda, AWS KMS, and the GitHub API.

The token issuance process follows these steps:

1. The Composite Action obtains a GitHub OIDC token and exchanges it for temporary AWS credentials through AWS STS.
2. The Composite Action obtains a separate GitHub OIDC token for GATES and invokes AWS Lambda with the token request.
3. AWS Lambda validates the OIDC token, identifies the caller, and authorizes the requested repositories and permissions.
4. AWS Lambda uses AWS KMS to sign a GitHub App JWT and requests an installation access token from the GitHub API.
5. AWS Lambda validates the issued token's scope and returns it to the Composite Action for use by subsequent workflow steps.

## Release notes

See [GitHub Releases][releases].

[releases]: https://github.com/tmknom/gates/releases
