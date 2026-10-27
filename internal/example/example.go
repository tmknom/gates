// Package example は、標準入力・カレントディレクトリ・起動時刻を JSON で出力するサンプル。
package example

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

// maxInputSize は、無制限の入力によるメモリ枯渇を避けるための上限。
const maxInputSize = 1_000_000

// Config は、Run の入力と出力先。
type Config struct {
	In      io.Reader
	Out     io.Writer
	WorkDir string
	Current time.Time
}

type output struct {
	In      string `json:"in"`
	WorkDir string `json:"work_dir"`
	Time    string `json:"time"`
}

// Run は、入力の読み込み中に ctx がキャンセルされた場合、ctx のエラーを返す。
func Run(ctx context.Context, cfg Config) error {
	in, err := readAll(ctx, cfg.In)
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}

	if !utf8.Valid(in) {
		return errors.New("input is not valid UTF-8")
	}

	out := output{
		In:      trimLineEnd(string(in)),
		WorkDir: cfg.WorkDir,
		Time:    cfg.Current.Format(time.RFC3339),
	}
	enc := json.NewEncoder(cfg.Out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

// trimLineEnd は、s の末尾の改行 (\n または \r\n) を 1 つだけ取り除く。単独の \r は改行とみなさない。
func trimLineEnd(s string) string {
	if t, ok := strings.CutSuffix(s, "\r\n"); ok {
		return t
	}
	return strings.TrimSuffix(s, "\n")
}

// readAll は、ブロック中の Read を中断できないため、
// キャンセル時は読み込み goroutine を残したまま戻る。
func readAll(ctx context.Context, r io.Reader) ([]byte, error) {
	type result struct {
		data []byte
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		data, err := io.ReadAll(io.LimitReader(r, maxInputSize+1))
		if err == nil && len(data) > maxInputSize {
			data, err = nil, fmt.Errorf("input exceeds %d bytes", maxInputSize)
		}
		ch <- result{data: data, err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		return res.data, res.err
	}
}
