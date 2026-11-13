package supervise_test

import (
	"bytes"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/supervise"
)

// childBehaviors は、試験の実行ファイルが子として起動されたときの振る舞いを、argv[0] の名前で選ぶ。
// 親が子の argv[0] に置く名前を変えられることを使い、本番と同じく argv[0] で振る舞いを選ぶ。
var childBehaviors = map[string]func(){
	"exit-0": func() { os.Exit(0) },
	"exit-1": func() { os.Exit(1) },
	// 回復されない panic は、Go のランタイムが値とスタックを標準エラー出力に書き、終了コード 2 で終える
	"panic-in-goroutine": func() {
		go func() { panic("panic-value-marker") }()
		select {}
	},
	"kill-self": func() {
		_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)
		select {}
	},
	// 受けたシグナルの名前を書いて終える。転送されなければ、試験が止まらないよう 5 秒で終了コード 3 で終える
	"report-signal": func() {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
		_, _ = fmt.Println("ready")
		select {
		case sig := <-signals:
			_, _ = fmt.Println(sig)
			os.Exit(0)
		case <-time.After(5 * time.Second):
			os.Exit(3)
		}
	},
	"print-env": func() {
		_, _ = fmt.Println(os.Getenv(inheritedEnvName))
		os.Exit(0)
	},
}

const (
	inheritedEnvName = "GATES_SUPERVISE_TEST_ENV"
	// childMarkerEnvName は、試験の実行ファイルが子として起動されたことを示す
	childMarkerEnvName = "GATES_SUPERVISE_TEST_CHILD"
)

func TestMain(m *testing.M) {
	if behave, ok := childBehaviors[filepath.Base(os.Args[0])]; ok {
		behave()
	}
	// 親が子の argv[0] を置き損ねると、子が振る舞いを選べずに試験全体を実行し、さらに子を起動し続ける。
	// 子は親の環境を引き継ぐため、この印があれば子として振る舞いを選べなかったとみなし、すぐに終える
	if os.Getenv(childMarkerEnvName) != "" {
		os.Exit(4)
	}
	if err := os.Setenv(childMarkerEnvName, "1"); err != nil {
		os.Exit(4)
	}
	os.Exit(m.Run())
}

func TestMain_ChildTermination(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		executable func(*testing.T) string
		childName  string
		wantCode   int
		wantStdout string
	}{
		{name: "child exiting 0 returns 0 without annotation", executable: testExecutable, childName: "exit-0", wantCode: 0, wantStdout: ""},
		{name: "child exiting 1 returns 1 without annotation", executable: testExecutable, childName: "exit-1", wantCode: 1, wantStdout: ""},
		{
			name: "child panicking in another goroutine returns 1 with the fixed error and no panic value", executable: testExecutable, childName: "panic-in-goroutine",
			wantCode: 1, wantStdout: "::error::the child process terminated unexpectedly\n",
		},
		{
			name: "child killed by a signal returns 1 with the fixed error", executable: testExecutable, childName: "kill-self",
			wantCode: 1, wantStdout: "::error::the child process terminated unexpectedly\n",
		},
		{
			name: "child that cannot start returns 1 with the fixed error", executable: missingExecutable, childName: "exit-0",
			wantCode: 1, wantStdout: "::error::the child process could not be started\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stdout := &bytes.Buffer{}
			cfg := supervise.Config{Executable: tt.executable(t), ChildName: tt.childName, Stdout: stdout}

			if got := supervise.Main(cfg, nil); got != tt.wantCode {
				t.Errorf("Main() = %d, want %d", got, tt.wantCode)
			}
			if got := stdout.String(); got != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", got, tt.wantStdout)
			}
		})
	}
}

//trace:checks AC-31
func TestPostEntrypoint_ChildTermination(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		executable func(*testing.T) string
		childName  string
		wantStdout string
	}{
		{name: "child exiting 0 returns 0 without annotation", executable: testExecutable, childName: "exit-0", wantStdout: ""},
		{
			name: "child exiting 1 returns 0 with the fixed warning", executable: testExecutable, childName: "exit-1",
			wantStdout: "::warning::the child process terminated unexpectedly\n",
		},
		{
			name: "child that cannot start returns 0 with the fixed warning", executable: missingExecutable, childName: "exit-0",
			wantStdout: "::warning::the child process could not be started\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stdout := &bytes.Buffer{}
			cfg := supervise.Config{Executable: tt.executable(t), ChildName: tt.childName, Stdout: stdout}

			if got := supervise.PostEntrypoint(cfg); got != 0 {
				t.Errorf("PostEntrypoint() = %d, want 0", got)
			}
			if got := stdout.String(); got != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", got, tt.wantStdout)
			}
		})
	}
}

//trace:checks AC-33
func TestMain_ForwardsSignalToChildAndWaits(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		sig  os.Signal
		want string
	}{
		{name: "SIGINT reaches the child", sig: syscall.SIGINT, want: "ready\ninterrupt\n"},
		{name: "SIGTERM reaches the child", sig: syscall.SIGTERM, want: "ready\nterminated\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stdout := &readyBuffer{ready: make(chan struct{})}
			signals := make(chan os.Signal, 1)
			go func() {
				<-stdout.ready
				signals <- tt.sig
			}()
			cfg := supervise.Config{Executable: testExecutable(t), ChildName: "report-signal", Stdout: stdout}

			if got := supervise.Main(cfg, signals); got != 0 {
				t.Errorf("Main() = %d, want 0", got)
			}
			if got := stdout.String(); got != tt.want {
				t.Errorf("stdout = %q, want %q", got, tt.want)
			}
		})
	}
}

// 環境変数を変えるため、ほかの試験と並行させない
func TestMain_ChildInheritsEnvironment(t *testing.T) {
	t.Setenv(inheritedEnvName, "inherited")
	stdout := &bytes.Buffer{}
	cfg := supervise.Config{Executable: testExecutable(t), ChildName: "print-env", Stdout: stdout}

	if got := supervise.Main(cfg, nil); got != 0 {
		t.Errorf("Main() = %d, want 0", got)
	}
	if got := stdout.String(); got != "inherited\n" {
		t.Errorf("stdout = %q, want %q", got, "inherited\n")
	}
}

// testExecutable は、子として起動する試験の実行ファイルのパスを返す。
func testExecutable(t *testing.T) string {
	t.Helper()
	path, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}
	return path
}

// missingExecutable は、起動できない、存在しないパスを返す。
func missingExecutable(t *testing.T) string {
	return filepath.Join(t.TempDir(), "missing")
}

// readyBuffer は、子が ready の行を書いた時点で ready を閉じる書き込み先である。
type readyBuffer struct {
	buf   bytes.Buffer
	ready chan struct{}
	once  sync.Once
}

func (b *readyBuffer) Write(p []byte) (int, error) {
	n, err := b.buf.Write(p)
	if strings.Contains(b.buf.String(), "ready\n") {
		b.once.Do(func() { close(b.ready) })
	}
	return n, err
}

func (b *readyBuffer) String() string {
	return b.buf.String()
}
