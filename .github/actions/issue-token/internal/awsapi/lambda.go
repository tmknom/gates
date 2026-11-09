package awsapi

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/credential"
)

// invokeTimeout の値の理由は、GitHub Actions Design の Retry and Timeout にある。
const invokeTimeout = 90 * time.Second

// LambdaClient は、AWS 一時認証情報で署名して Lambda を Invoke する。
type LambdaClient struct {
	client *lambda.Client
}

// NewLambdaClient は、region の Lambda に creds で署名して要求する LambdaClient を作る。
// 共有の設定ファイルと環境変数を読まず、再試行しない（GitHub Actions Design の Retry and Timeout）。
func NewLambdaClient(region string, creds credential.AWSCredentials, opts ...Option) *LambdaClient {
	o := newOptions(opts)
	lambdaOptions := lambda.Options{
		Region:       region,
		Retryer:      aws.NopRetryer{},
		BaseEndpoint: o.endpoint(),
		Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     creds.AccessKeyID.Expose(),
				SecretAccessKey: creds.SecretAccessKey.Expose(),
				SessionToken:    creds.SessionToken.Expose(),
			}, nil
		}),
	}
	if o.httpClient != nil {
		lambdaOptions.HTTPClient = o.httpClient
	}
	return &LambdaClient{client: lambda.New(lambdaOptions)}
}

// Invoke は、functionName を payload で同期的に Invoke する。
// FunctionError と 200 以外の StatusCode は error にせず、Invoke の結果として返す。
func (c *LambdaClient) Invoke(ctx context.Context, functionName string, payload []byte) (InvokeResult, error) {
	ctx, cancel := context.WithTimeout(ctx, invokeTimeout)
	defer cancel()

	out, err := c.client.Invoke(ctx, &lambda.InvokeInput{
		FunctionName: aws.String(functionName),
		// InvocationType は省略しない（GitHub Actions Design の Processing Flow）
		InvocationType: types.InvocationTypeRequestResponse,
		Payload:        payload,
	})
	if err != nil {
		return InvokeResult{}, classify(err)
	}
	return InvokeResult{
		StatusCode:    int(out.StatusCode),
		FunctionError: aws.ToString(out.FunctionError),
		Payload:       out.Payload,
	}, nil
}
