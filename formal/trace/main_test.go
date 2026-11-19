package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		files      map[string]string
		want       int
		wantStdout string
	}{
		{name: "repository whose links all hold exits with 0", want: 0},
		{
			name:       "findings are written to stdout and exit with 1",
			files:      map[string]string{"docs/spec/x.md": lines("GOJ-SUB")},
			want:       1,
			wantStdout: "docs/spec/x.md:1: undefined ID GOJ-SUB\n",
		},
		{name: "map is written to stdout and exits with 0", args: []string{"-map"}, want: 0, wantStdout: "| ID | Formalized by | Unformalized at | Checked by |"},
		{name: "unknown flag exits with 2", args: []string{"-unknown"}, want: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(tc.args, repository(tc.files), &stdout, &stderr); got != tc.want {
				t.Errorf("run() = %d, want %d (stderr %q)", got, tc.want, stderr.String())
			}
			if !strings.HasPrefix(stdout.String(), tc.wantStdout) {
				t.Errorf("stdout = %q, want a prefix %q", stdout.String(), tc.wantStdout)
			}
		})
	}
}
