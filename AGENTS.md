# AGENTS.md

GATES issues GitHub App installation access tokens to GitHub Actions workflows via GitHub OIDC.
Built with Go and Terraform (AWS Lambda, KMS, IAM), verified with Alloy.

## Rules

- Read `llms.txt` at task start, then read the documents the task needs
- Write commit messages and PR descriptions in English
- Security first: when security conflicts with anything else, security wins
- Test first: write a failing test before the code
- Keep code, docs, and `formal/` models in sync within the same change
- Use the domain model's terms in code, docs, and models alike, so they never contradict each other
- Verify every deliverable; verify specs and designs with formal methods where feasible
