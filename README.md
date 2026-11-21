# GATES

**GitHub App Token Exchange System**

Give your GitHub Actions workflows short-lived access to other repositories, limited to the repositories and permissions they ask for, without ever handing them a GitHub App private key.

## Why GATES

The `GITHUB_TOKEN` that GitHub Actions provides can only reach the repository the workflow runs in.
To read or update another repository, such as a shared configuration repository or a private dependency, teams usually reach for one of two workarounds:

- a personal access token stored as a secret, tied to one person's account and often broader and longer-lived than needed
- a GitHub App private key stored as a secret, which lets anyone who obtains it act with every permission of the GitHub App until someone deletes the key from the GitHub App

GATES replaces both.
A workflow asks GATES for the repositories and permissions it needs and receives a GitHub App installation access token for only those repositories and permissions.

## Benefits

- **No long-lived secrets in workflows**: Workflows prove who they are with GitHub OIDC. There is no private key, personal access token, or AWS access key to store, rotate, or leak.
- **Private key locked in AWS KMS**: The GitHub App private key is imported into AWS KMS and signs only inside it, so no workflow, runner, secret, or even the GATES Lambda function holds the key.
- **Least-privilege tokens**: Each token covers only the repositories and the repository permissions you list, such as `contents:write` and `pull_requests:read`. GATES never widens a request on its own, and it rejects the whole request if any listed repository or permission is not allowed.
- **Short-lived tokens, revoked when the job ends**: Tokens expire on GitHub's schedule (one hour), and the action also tries to revoke them when the job ends, even after a failed or canceled step.
- **Fail closed**: If GATES cannot verify the caller, authorize the request, or confirm that the token GitHub issued matches the request, the workflow gets no token.
- **Auditable**: Every request that GATES answers leaves an audit record of its result, with the requested repositories and the calling repository and workflow run once GATES has checked them, never the token itself. Every response carries a request ID that points to its record.
- **One step to use**: Workflow authors add one action and read one output. Authentication, signing, and issuance stay inside GATES.

## How it works

```mermaid
sequenceDiagram
    participant W as Workflow (issue-token action)
    participant S as AWS STS
    participant L as GATES (AWS Lambda)
    participant K as AWS KMS
    participant G as GitHub API

    W->>S: Exchange a GitHub OIDC token for temporary AWS credentials
    W->>L: Request a token with a second GitHub OIDC token
    L->>L: Verify the caller and authorize the repositories and permissions
    L->>K: Sign a GitHub App JWT
    L->>G: Request an installation access token
    L->>L: Check that the token matches the request
    L-->>W: Return the token
    Note over W,G: At the end of the job, the action tries to revoke the token
```

GATES trusts a caller by its immutable GitHub owner ID, not by its name, so renaming or re-creating an account cannot impersonate an allowed owner.

## Usage

### Prerequisites

- The GATES operator has deployed GATES for your organization or user account and registered its connection information as GitHub Secrets and Variables (`GATES_AWS_ACCOUNT_ID`, `GATES_ROLE_NAME`, `GATES_FUNCTION_NAME`, `GATES_AWS_REGION`, `GATES_AUDIENCE`).
- The repositories you request belong to the same owner as the calling repository, and the GATES GitHub App is installed on them.
- The permissions you request are granted to the GATES GitHub App and included in the issuable permissions, the list of permissions the operator allows the deployment to issue.
- The calling repository uses GitHub's immutable OIDC subject claims. Repositories created after 2026-07-15 use them by default; older repositories need `use_immutable_subject` enabled on the organization or repository.
- The job declares `id-token: write`.
- The job runs on a Linux runner with Docker, such as `ubuntu-latest`. Self-hosted runners also need the network access listed in [Using GATES in Workflows](docs/user-guide/workflow-usage.md#6-self-hosted-runners).

### Example

```yaml
name: Read another repository
on: workflow_dispatch

jobs:
  read-repository:
    runs-on: ubuntu-latest
    permissions:
      id-token: write
    steps:
      - name: Issue token
        id: gates
        uses: tmknom/gates/actions/issue-token@<PINNED_COMMIT_SHA>
        with:
          repositories: example-org/repo-a,example-org/repo-b
          permissions: contents:read
          role-arn: arn:aws:iam::${{ secrets.GATES_AWS_ACCOUNT_ID }}:role/${{ vars.GATES_ROLE_NAME }}
          function-name: ${{ vars.GATES_FUNCTION_NAME }}
          aws-region: ${{ vars.GATES_AWS_REGION }}
          audience: ${{ vars.GATES_AUDIENCE }}

      - name: Read a file from repo-a
        env:
          GH_TOKEN: ${{ steps.gates.outputs.token }}
        run: gh api repos/example-org/repo-a/contents/README.md
```

Pin the action to a full commit SHA rather than a tag or branch, because a tag or branch can later point to a different commit.
Use the commit of the release your GATES operator deployed, because combinations of different releases of the action and the deployed GATES are not verified.

### Inputs

All inputs are required.

| Input | Description |
|---|---|
| `repositories` | Comma-separated `owner/repo` list of the repositories the token can access (up to 500) |
| `permissions` | Comma-separated `permission:level` list of repository permissions, such as `contents:read,pull_requests:write`. The accepted names and levels are listed in the [Issue Token action](actions/issue-token/README.md) |
| `role-arn` | ARN of the IAM role to assume. Build it from the operator's Secrets and Variables as in the example |
| `function-name` | Name of the GATES Lambda function |
| `aws-region` | AWS region of the GATES Lambda function |
| `audience` | Audience of the OIDC token sent to GATES |

### Outputs

| Output | Description |
|---|---|
| `token` | The installation access token, masked in logs |
| `expires-at` | Expiration time of the token, in RFC 3339 |
| `repositories` | Comma-separated `owner/repo` list the token can access |
| `permissions` | Comma-separated `permission:level` list the token grants. It may include `metadata:read`, which GitHub adds automatically |

### When the action fails

The action fails the step and writes the cause as an `::error::` annotation.
When GATES itself rejects the request, the annotation reads `GATES returned a failure response` and includes an error code, a message, and a `request_id`.
Share the `request_id` with your GATES operator so they can find the matching audit record.

If the action cannot revoke the token at the end of the job, it writes a `::warning::` annotation but does not fail the job.
The token still stops working at `expires-at`.

## Security notes

- Use the `token` output only in later steps of the same job. GitHub refuses to pass masked values through job `outputs`.
- Request only the permissions the job needs, at the lowest level that works.
- Do not run untrusted code, such as pull request code checked out under `pull_request_target`, in a job that requests a token. Any code in that job can read the token you received and request its own token from GATES, even with permissions you did not request.
- Changes made with a token, such as pushed commits or added deploy keys, remain after the token expires or is revoked.
- Workflows triggered by Dependabot cannot read GitHub Actions secrets. Register `GATES_AWS_ACCOUNT_ID` as a Dependabot secret too if Dependabot workflows need tokens.
- The action masks the token, but not a substring of it that you print yourself.

## Scope

GATES issues tokens with repository permissions for repositories of a single allowed owner to GitHub Actions workflows of that owner.
It does not issue organization, account, or enterprise permissions.
It does not serve clients outside GitHub Actions, manage other kinds of secrets, or change GitHub App installations and permissions for you.

Authorization is currently per owner: any workflow of the allowed owner can request any permission that is both granted to the GitHub App and in the issuable permissions, for any repository the GitHub App is installed on.
Grant the GitHub App and the issuable permissions only what the workflows need, and keep this in mind before installing the GitHub App on highly sensitive repositories.
Write permissions such as `workflows`, `administration`, `secrets`, and `actions` can reach the secrets and settings of other repositories.
Finer-grained authorization per caller or team, and organization, account, and enterprise permissions, are candidates for the future, with no timeline yet.

## Documentation

- [User Guide](docs/user-guide/README.md) (in Japanese): For people who evaluate, deploy, operate, or use GATES from workflows. Overview, security considerations, a deployment tutorial, key material import and rotation, release verification, writing workflows, operations, and troubleshooting
- [Docs](docs/README.md): For people who develop GATES. Purpose and design principles, requirements, design, research on GitHub and AWS behavior, and development procedures such as testing and releases
- [Issue Token action](actions/issue-token/README.md): Detailed reference for the action, including input validation and every failure annotation
- [Formal models](formal/README.md): Models that check the authorization and token issuance rules
- [llms.txt](llms.txt): Index of the repository for coding agents

## License

[Apache License 2.0](LICENSE)

## Release notes

See [GitHub Releases](https://github.com/tmknom/gates/releases).

Each release includes the Lambda deployment package `gates-lambda-linux-arm64.zip`, its `SHA256SUMS`, and a build provenance attestation.
To verify the package, see [Release Verification](docs/user-guide/verify-release.md).
