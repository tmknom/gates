// Package issue は、メインの入口関数を提供する。前提の確認から Output の書き込みまでの段階を順に行い（GitHub Actions Design の Processing Flow）、
// Installation Access Token を後続のステップに渡す。
package issue

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/awsapi"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/githubapi"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/oidc"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/response"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/revocation"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/workflow"
)

// errCanceled は、キャンセルの要求を受けて終えたことを表す。
var errCanceled = errors.New("the job was canceled")

// Config は、メインの入口関数の設定である。
type Config struct {
	// 入力の値。空かどうかは Run が確かめる
	Repositories, Permissions, RoleARN, FunctionName, AWSRegion, Audience string

	// OIDCRequestURL は ACTIONS_ID_TOKEN_REQUEST_URL の値である。
	OIDCRequestURL string
	// OIDCRequestToken は ACTIONS_ID_TOKEN_REQUEST_TOKEN の値で、Run が最初にマスクする。
	OIDCRequestToken string
	// OutputPath は GITHUB_OUTPUT の値である。
	OutputPath string
	// StatePath は GITHUB_STATE の値である。
	StatePath string

	// Stdout は、ワークフローコマンドとログの書き込み先である。
	Stdout io.Writer
	// Now は、Invoke の時刻をログに出すための現在時刻を返す。
	Now func() time.Time

	// 外部クライアントの Option。テストが httptest のサーバーに向けるために使う
	OIDC   []oidc.Option
	STS    []awsapi.Option
	Lambda []awsapi.Option
	GitHub []githubapi.Option
}

// Run は、前提の確認から Output の書き込みまでを順に行う。
// 失敗したら、段階を示す文言を ::error:: で Stdout に書き、その error を返す（GitHub Actions Design の Failure Output）。
// ctx のキャンセルをキャンセルの要求として受け、受けた時点に応じて終える（GitHub Actions Design の Cancellation）。
func Run(ctx context.Context, cfg Config) error {
	err := run(ctx, cfg)
	if err == nil {
		return nil
	}
	return errors.Join(err, workflow.Error(cfg.Stdout, err.Error()))
}

func run(ctx context.Context, cfg Config) error {
	prepared, err := prepare(ctx, cfg)
	if err != nil {
		// 要求を受けた後の失敗は、通信の打ち切りによるものとして、段階の文言ではなくキャンセルとして報告する
		if ctx.Err() != nil {
			return errCanceled
		}
		return err
	}
	// Invoke の前の要求は、Lambda を呼ばずに終える。トークンはまだ発行されていない
	if ctx.Err() != nil {
		return errCanceled
	}

	// Invoke の時刻をログに出す（GitHub Actions Design の Failure Output）
	if _, err := fmt.Fprintf(cfg.Stdout, "Invoking the Lambda function at %s\n", cfg.Now().UTC().Format(time.RFC3339)); err != nil {
		return err
	}
	// Invoke はキャンセルの要求で打ち切らない（GitHub Actions Design の Cancellation）
	lambdaClient := awsapi.NewLambdaClient(cfg.AWSRegion, prepared.creds, cfg.Lambda...)
	success, err := response.Judge(lambdaClient.Invoke(context.WithoutCancel(ctx), cfg.FunctionName, prepared.payload))
	if err != nil {
		// トークンを取り出す前の失敗のため、失効するトークンはない
		return err
	}

	if err := deliver(ctx, cfg, success); err != nil {
		// 失敗時の失効も、キャンセルの要求で打ち切らない（GitHub Actions Design の Revocation）
		revocationErr := revocation.Attempt(context.WithoutCancel(ctx), cfg.Stdout, githubapi.NewClient(cfg.GitHub...), success.Token())
		return errors.Join(err, revocationErr)
	}
	return nil
}
