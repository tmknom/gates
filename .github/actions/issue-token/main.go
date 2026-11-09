// Command issue-token は、GATES からトークンを受け取る Docker Action のプログラムである。
// 起動したパスの末尾の名前で、親と子、メインと post-entrypoint を選ぶ（GitHub Actions Design の Packages）。
package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/issue"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/revoke"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/supervise"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/workflow"
)

// 起動したパスの末尾の名前。親は、自身の名前に -child を付けた名前で子を起動する
const (
	mainParent = "issue-token"
	postParent = "revoke-token"
	mainChild  = mainParent + "-child"
	postChild  = postParent + "-child"
)

// unexpectedFailureMessage は、子が回復した panic の注釈である。panic の値は出さない（GitHub Actions Design の Failure Output）
const unexpectedFailureMessage = "an unexpected failure occurred"

func main() {
	os.Exit(run())
}

func run() int {
	switch filepath.Base(os.Args[0]) {
	case mainParent:
		return runMainParent()
	case postParent:
		return runPostParent()
	case mainChild:
		return runMainChild()
	case postChild:
		runPostChild()
		return 0
	default:
		return 1
	}
}

// runMainParent は、シグナルの受け口を作り、メインの子の起動と見張りを呼ぶ。親は環境変数を読まない。
func runMainParent() int {
	// SIGINT と SIGTERM を受け口に流し、Go のランタイムが親を終えないようにする
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	return supervise.Main(childConfig(mainChild), signals)
}

// runPostParent は、post-entrypoint の子の起動と見張りを呼ぶ。親は環境変数を読まない。
// SIGINT と SIGTERM を無視し、子へ送らない（GitHub Actions Design の Cancellation）
func runPostParent() int {
	signal.Ignore(syscall.SIGINT, syscall.SIGTERM)
	return supervise.PostEntrypoint(childConfig(postChild))
}

// childConfig は、自身の実行ファイルを childName の子として起動する設定を返す。
// Runner は Linux に限るため、実行ファイル自身を /proc/self/exe で指す（GitHub Actions Design の Runner Requirements）
func childConfig(childName string) supervise.Config {
	return supervise.Config{Executable: "/proc/self/exe", ChildName: childName, Stdout: os.Stdout}
}

// runMainChild は、メインの入口関数を呼ぶ。SIGINT と SIGTERM を、ctx のキャンセルとしてキャンセルの要求にする。
func runMainChild() (code int) {
	defer func() {
		if recover() == nil {
			return
		}
		_ = workflow.Error(os.Stdout, unexpectedFailureMessage)
		code = 1
	}()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	// 失敗の注釈は issue.Run が書いている
	if err := issue.Run(ctx, issue.Config{
		Repositories:     os.Getenv("INPUT_REPOSITORIES"),
		Permissions:      os.Getenv("INPUT_PERMISSIONS"),
		RoleARN:          os.Getenv("INPUT_ROLE-ARN"),
		FunctionName:     os.Getenv("INPUT_FUNCTION-NAME"),
		AWSRegion:        os.Getenv("INPUT_AWS-REGION"),
		Audience:         os.Getenv("INPUT_AUDIENCE"),
		OIDCRequestURL:   os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL"),
		OIDCRequestToken: os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN"),
		OutputPath:       os.Getenv("GITHUB_OUTPUT"),
		StatePath:        os.Getenv("GITHUB_STATE"),
		Stdout:           os.Stdout,
		Now:              time.Now,
	}); err != nil {
		return 1
	}
	return 0
}

// runPostChild は、post-entrypoint の入口関数を呼ぶ。失効を終えてから終えるため、SIGINT と SIGTERM を無視する。
// panic を回復しても、警告にとどめて終了コード 0 で終える（GitHub Actions Design の Failure Output）。
func runPostChild() {
	defer func() {
		if recover() == nil {
			return
		}
		_ = workflow.Warning(os.Stdout, unexpectedFailureMessage)
	}()
	signal.Ignore(syscall.SIGINT, syscall.SIGTERM)
	// 返る error は標準出力への書き込みの失敗だけで、知らせる先がない
	_ = revoke.Run(context.Background(), revoke.Config{Token: os.Getenv("STATE_token"), Stdout: os.Stdout})
}
