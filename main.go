package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tmknom/template-go/cmd"
)

// ビルド時に -ldflags "-X main.version=... -X main.commit=..." で埋め込む。
var (
	version = ""
	commit  = ""
)

const exitFailure = 1

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: get working directory: %v\n", err)
		return exitFailure
	}

	env := cmd.Env{
		Stdin:   os.Stdin,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Current: time.Now().UTC(),
		WorkDir: wd,
		Version: version,
		Commit:  commit,
	}
	return cmd.Execute(ctx, os.Args[1:], env)
}
