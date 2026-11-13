module github.com/tmknom/gates

// go の行は言語の下限で、golangci-lint のイメージのように Go を同梱するツールもこれを満たす必要がある
go 1.27.0

// リリースビルドに使う Go は、setup-go がこの行から決める。一致しなければ reusable-build.yml のビルドが失敗する
toolchain go1.27.4

require (
	github.com/aws/aws-lambda-go v1.55.1
	github.com/aws/aws-sdk-go-v2 v1.47.8
	github.com/aws/aws-sdk-go-v2/config v1.33.16
	github.com/aws/aws-sdk-go-v2/service/kms v1.62.5
	github.com/aws/smithy-go v1.28.9
	github.com/coreos/go-oidc/v3 v3.21.0
	github.com/go-jose/go-jose/v4 v4.1.9
)

require (
	github.com/aws/aws-sdk-go-v2/credentials v1.20.16 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.20.8 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.11 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.11 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.11 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.24 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.11 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.12.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.39.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.44.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.51.9 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	golang.org/x/mod v0.42.0 // indirect
	golang.org/x/oauth2 v0.36.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/telemetry v0.0.0-20261010193011-f71dc92da45e // indirect
	golang.org/x/tools v0.52.0 // indirect
	golang.org/x/vuln v1.9.0 // indirect
)

tool (
	golang.org/x/tools/cmd/goimports
	golang.org/x/vuln/cmd/govulncheck
)
