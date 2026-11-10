// Package supervise は、親の処理を提供する。同じ実行ファイルを子として起動し、
// 子の終わり方から親の終了コードと注釈を決める（GitHub Actions Design の Failure Output と Cancellation）。
package supervise

import (
	"io"
	"os"
	"os/exec"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/workflow"
)

// 注釈の文言は固定し、子の終了コード、シグナル、panic の値を含めない（GitHub Actions Design の Failure Output）
const (
	startFailureMessage          = "the child process could not be started"
	unexpectedTerminationMessage = "the child process terminated unexpectedly"
)

// Config は、親が子を起動するための設定である。
type Config struct {
	// Executable は、子として起動する実行ファイルのパスである。
	Executable string
	// ChildName は、子の argv[0] に置く名前である。
	ChildName string
	// Stdout は、子が引き継ぐ標準出力で、親の注釈の書き込み先でもある。
	Stdout io.Writer
}

// Main は、メインの親として子を起動し、signals で受けたシグナルを子へ送りながら終了を待ち、親の終了コードを返す。
func Main(cfg Config, signals <-chan os.Signal) int {
	exitCode, started := supervise(cfg, signals)
	var message string
	switch {
	case !started:
		message = startFailureMessage
	// メインの子のコードが使う終了コードは 0 と 1 だけで、それ以外は想定外の終わり方である
	case exitCode == 0 || exitCode == 1:
		return exitCode
	default:
		message = unexpectedTerminationMessage
	}
	// 終了コードが親の結果で、注釈を書けなかったことを知らせる先はほかにない
	_ = workflow.Error(cfg.Stdout, message)
	return 1
}

// PostEntrypoint は、post-entrypoint の親として子を起動して終了を待ち、0 を返す。
// 子が想定外に終わってもジョブを失敗させず、::warning:: で知らせる（GitHub Actions Design の Failure Output）。
func PostEntrypoint(cfg Config) int {
	exitCode, started := supervise(cfg, nil)
	var message string
	switch {
	case !started:
		message = startFailureMessage
	// post-entrypoint の子のコードが使う終了コードは 0 だけで、それ以外は想定外の終わり方である
	case exitCode == 0:
		return 0
	default:
		message = unexpectedTerminationMessage
	}
	// 終了コードが親の結果で、注釈を書けなかったことを知らせる先はほかにない
	_ = workflow.Warning(cfg.Stdout, message)
	return 0
}

// supervise は、子を起動し、signals で受けたシグナルを子へ送りながら終了を待ち、子の終了コードを返す。
// シグナルによる終了の終了コードは -1 である。起動できなければ started は false である。
func supervise(cfg Config, signals <-chan os.Signal) (exitCode int, started bool) {
	// Stderr を nil にして /dev/null につなぎ、Go のランタイムが書く panic の値とスタックを捨てる。
	// Env を nil にして親の環境を引き継ぐ
	child := &exec.Cmd{Path: cfg.Executable, Args: []string{cfg.ChildName}, Stdout: cfg.Stdout}
	if err := child.Start(); err != nil {
		return 0, false
	}
	// 親が先に終わると、同じコンテナの子もカーネルに強制終了されるため、シグナルを受けても待ち続ける
	done := make(chan struct{})
	go func() {
		// 終了コードは ProcessState から読む。Wait の error の文言は注釈に出さない
		_ = child.Wait()
		close(done)
	}()
	for {
		select {
		case sig := <-signals:
			// 子が終わった直後は送れないが、その終了は done で受け取る
			_ = child.Process.Signal(sig)
		case <-done:
			return child.ProcessState.ExitCode(), true
		}
	}
}
