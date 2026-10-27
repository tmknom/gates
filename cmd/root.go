// Package cmd は CLI 層。コマンドを組み立て、実行結果から終了コードを決める。
package cmd

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/tmknom/template-go/internal/example"
)

const (
	exitOK      = 0
	exitFailure = 1
	// exitInterrupted は、シグナルによる中断の慣習的な終了コード (128 + SIGINT)。
	exitInterrupted = 130
)

// Execute は args を実行し、終了コードを返す。
func Execute(ctx context.Context, args []string, env Env) int {
	root := newRootCmd(ctx, env)
	root.SetArgs(args)
	root.SetIn(env.Stdin)
	root.SetOut(env.Stdout)
	root.SetErr(env.Stderr)

	if err := root.ExecuteContext(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			return exitInterrupted
		}
		return exitFailure
	}
	return exitOK
}

func newRootCmd(ctx context.Context, env Env) *cobra.Command {
	return &cobra.Command{
		Use:          "template-go",
		Short:        "Template repository for Go",
		Version:      env.version(),
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(c *cobra.Command, _ []string) error {
			return example.Run(ctx, example.Config{
				In:      c.InOrStdin(),
				Out:     c.OutOrStdout(),
				WorkDir: env.WorkDir,
				Current: env.Current,
			})
		},
	}
}
