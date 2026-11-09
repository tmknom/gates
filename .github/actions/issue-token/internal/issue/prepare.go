package issue

import (
	"context"
	"fmt"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/awsapi"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/credential"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/oidc"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/request"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/workflow"
)

// awsAudience は、AWS STS が AssumeRoleWithWebIdentity で受け付ける OIDC JWT の Audience である。
const awsAudience = "sts.amazonaws.com"

// invocation は、Invoke に要る Payload と、署名に使う AWS 一時認証情報である。
type invocation struct {
	payload []byte
	creds   credential.AWSCredentials
}

// prepare は、Invoke の前の段階を順に行う。資格情報は、得た直後に、ほかの出力より先にマスクする（GitHub Actions Design の Masking）。
func prepare(ctx context.Context, cfg Config) (invocation, error) {
	requestToken := credential.NewOIDCRequestToken(cfg.OIDCRequestToken)
	// 空の要求用トークンは前提の確認で失敗し、隠す値がないため、マスクしない
	if cfg.OIDCRequestToken != "" {
		if err := workflow.AddMask(cfg.Stdout, requestToken); err != nil {
			return invocation{}, err
		}
	}
	if err := checkPrerequisites(cfg); err != nil {
		return invocation{}, err
	}
	oidcClient, err := oidc.NewClient(cfg.OIDCRequestURL, requestToken, cfg.OIDC...)
	if err != nil {
		return invocation{}, fmt.Errorf("invalid ACTIONS_ID_TOKEN_REQUEST_URL: %w", err)
	}
	tokenRequest, err := request.Parse(cfg.Repositories, cfg.Permissions)
	if err != nil {
		return invocation{}, fmt.Errorf("invalid inputs:\n%w", err)
	}

	awsJWT, err := oidcClient.Retrieve(ctx, awsAudience)
	if err != nil {
		return invocation{}, fmt.Errorf("could not get the OIDC JWT for AWS STS: %w", err)
	}
	if err := workflow.AddMask(cfg.Stdout, awsJWT); err != nil {
		return invocation{}, err
	}
	creds, err := awsapi.NewSTSClient(cfg.AWSRegion, cfg.STS...).AssumeRoleWithWebIdentity(ctx, cfg.RoleARN, awsJWT)
	if err != nil {
		return invocation{}, fmt.Errorf("AssumeRoleWithWebIdentity failed: %w", err)
	}
	if err := workflow.AddMask(cfg.Stdout, creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken); err != nil {
		return invocation{}, err
	}
	gatesJWT, err := oidcClient.Retrieve(ctx, cfg.Audience)
	if err != nil {
		return invocation{}, fmt.Errorf("could not get the OIDC JWT for GATES: %w", err)
	}
	if err := workflow.AddMask(cfg.Stdout, gatesJWT); err != nil {
		return invocation{}, err
	}
	return invocation{payload: tokenRequest.Payload(gatesJWT), creds: creds}, nil
}
