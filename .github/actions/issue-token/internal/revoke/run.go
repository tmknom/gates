// Package revoke は、post-entrypoint の入口関数を提供する。メインの処理が GITHUB_STATE に書いたトークンの、ジョブの終了時の失効を試みる。
package revoke

import (
	"context"
	"errors"
	"io"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/credential"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/githubapi"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/revocation"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/workflow"
)

// Config は、post-entrypoint の入口関数の設定である。
type Config struct {
	// Token は STATE_token の値で、メインの処理が GITHUB_STATE に書く前に終えたジョブでは空である。
	Token  string
	Stdout io.Writer
	GitHub []githubapi.Option
}

// Run は、トークンがあればマスクして失効を試み、結果を Stdout に書く。
// 失効の失敗は警告にとどめ、error にしない（GitHub Actions Design の Revocation）。返す error は Stdout への書き込みの失敗だけである。
// マスクを書けなくても失効は試みる。失効の要求と結果の出力はトークンの値を出さず、失効させることのほうがトークンを残さないため。
func Run(ctx context.Context, cfg Config) error {
	if cfg.Token == "" {
		return nil
	}
	token := credential.NewInstallationAccessToken(cfg.Token)
	maskErr := workflow.AddMask(cfg.Stdout, token)
	return errors.Join(maskErr, revocation.Attempt(ctx, cfg.Stdout, githubapi.NewClient(cfg.GitHub...), token))
}
