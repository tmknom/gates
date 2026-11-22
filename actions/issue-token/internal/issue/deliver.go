package issue

import (
	"context"
	"errors"
	"fmt"

	"github.com/tmknom/gates/actions/issue-token/internal/response"
	"github.com/tmknom/gates/actions/issue-token/internal/revocation"
	"github.com/tmknom/gates/actions/issue-token/internal/workflow"
)

// deliver は、取り出したトークンをマスクし、残りの項目を判定して、トークンを GITHUB_STATE でジョブの終了時の失効へ引き継ぎ、
// Output を GITHUB_OUTPUT に書いて後続ステップへ渡す。
// error を返したら、呼び出し側がトークンを失効させる。
func deliver(ctx context.Context, cfg Config, success response.Success) error {
	token := success.Token()
	if err := workflow.AddMask(cfg.Stdout, token); err != nil {
		return err
	}
	// キャンセルの要求を確かめるのはここが最後である（GitHub Actions Design の Cancellation）
	if ctx.Err() != nil {
		return errCanceled
	}
	items, err := success.Items()
	if err != nil {
		return err
	}

	// GITHUB_STATE を先に書く（GitHub Actions Design の Inputs and Outputs）。
	// ライブラリの error はファイルのパスを含み得るため、文言には環境変数の名前だけを出す
	if workflow.AppendFileCommand(cfg.StatePath, workflow.FileCommand{Name: revocation.TokenStateName, Value: token.Expose()}) != nil {
		return errors.New("could not write to GITHUB_STATE")
	}
	if workflow.AppendFileCommand(cfg.OutputPath,
		workflow.FileCommand{Name: "token", Value: token.Expose()},
		workflow.FileCommand{Name: "expires-at", Value: items.ExpiresAt},
		workflow.FileCommand{Name: "repositories", Value: items.Repositories},
		workflow.FileCommand{Name: "permissions", Value: items.Permissions},
	) != nil {
		return errors.New("could not write to GITHUB_OUTPUT")
	}
	_, err = fmt.Fprintf(cfg.Stdout, "Issued the token: request_id=%s expires_at=%s repositories=%s permissions=%s\n",
		items.RequestID, items.ExpiresAt, items.Repositories, items.Permissions)
	return err
}
