package awsapi

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/credential"
)

const (
	// roleSessionName は、AssumeRoleWithWebIdentity のセッション名である。
	roleSessionName = "gates-issue-token"
	// assumeRoleDuration は、AWS 一時認証情報の有効期間（秒）で、STS が受け付ける最小値である。
	assumeRoleDuration = 900
	// assumeRoleTimeout の値の理由は、GitHub Actions Design の Retry and Timeout にある。
	assumeRoleTimeout = 30 * time.Second
)

var errIncompleteCredentials = errors.New("the AWS STS response lacks AWS credentials")

// STSClient は、AWS STS で IAM Role を引き受ける。
type STSClient struct {
	client *sts.Client
}

// NewSTSClient は、region の AWS STS に無署名で要求する STSClient を作る。
// 共有の設定ファイルと環境変数を読まず、再試行しない（GitHub Actions Design の Retry and Timeout）。
func NewSTSClient(region string, opts ...Option) *STSClient {
	o := newOptions(opts)
	stsOptions := sts.Options{
		Region:       region,
		Retryer:      aws.NopRetryer{},
		BaseEndpoint: o.endpoint(),
	}
	if o.httpClient != nil {
		stsOptions.HTTPClient = o.httpClient
	}
	return &STSClient{client: sts.New(stsOptions)}
}

// AssumeRoleWithWebIdentity は、roleARN の IAM Role を token で引き受け、AWS 一時認証情報を返す。
func (c *STSClient) AssumeRoleWithWebIdentity(ctx context.Context, roleARN string, token credential.OIDCJWT) (credential.AWSCredentials, error) {
	ctx, cancel := context.WithTimeout(ctx, assumeRoleTimeout)
	defer cancel()

	out, err := c.client.AssumeRoleWithWebIdentity(ctx, &sts.AssumeRoleWithWebIdentityInput{
		RoleArn:          aws.String(roleARN),
		RoleSessionName:  aws.String(roleSessionName),
		WebIdentityToken: aws.String(token.Expose()),
		DurationSeconds:  aws.Int32(assumeRoleDuration),
	})
	if err != nil {
		return credential.AWSCredentials{}, classify(err)
	}
	if out.Credentials == nil || out.Credentials.AccessKeyId == nil || out.Credentials.SecretAccessKey == nil || out.Credentials.SessionToken == nil {
		return credential.AWSCredentials{}, errIncompleteCredentials
	}
	return credential.NewAWSCredentials(*out.Credentials.AccessKeyId, *out.Credentials.SecretAccessKey, *out.Credentials.SessionToken), nil
}
