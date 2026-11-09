module github.com/tmknom/gates/.github/actions/issue-token

go 1.27.0

tool (
	golang.org/x/tools/cmd/goimports
	golang.org/x/vuln/cmd/govulncheck
)

require (
	github.com/aws/aws-sdk-go-v2 v1.47.3
	github.com/aws/aws-sdk-go-v2/service/lambda v1.113.2
	github.com/aws/aws-sdk-go-v2/service/sts v1.51.4
	github.com/aws/smithy-go v1.28.5
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.23 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.6 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.6 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.21 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.6 // indirect
	golang.org/x/mod v0.42.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/telemetry v0.0.0-20261010193011-f71dc92da45e // indirect
	golang.org/x/tools v0.52.0 // indirect
	golang.org/x/vuln v1.9.0 // indirect
)
