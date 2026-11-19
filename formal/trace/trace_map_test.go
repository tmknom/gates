package main

import (
	"bytes"
	"testing"
)

func TestWriteMap(t *testing.T) {
	l, err := read(repository(map[string]string{
		"internal/x/z_test.go": goTest("//trace:checks GOJ-AUD RFC7519-AUD-REJECT"),
	}))
	if err != nil {
		t.Fatalf("read() error = %v", err)
	}
	var out bytes.Buffer
	if err := writeMap(&out, l); err != nil {
		t.Fatalf("writeMap() error = %v", err)
	}
	want := lines(
		"| ID | Formalized by | Unformalized at | Checked by |",
		"|---|---|---|---|",
		"| GH-OIDC-AUD-CUSTOM | formal/z3/model/github.smt2:1 |  | formal/z3/checks/compat/aud-custom.smt2:2 |",
		"| GH-OIDC-AUD-X |  |  | formal/z3/checks/compat/aud-x.smt2:2 |",
		"| GOJ-AUD | formal/z3/model/gates.smt2:1 |  | formal/z3/checks/compat/aud-custom.smt2:2<br>formal/z3/checks/compat/aud-x.smt2:2<br>formal/z3/checks/safety/aud.smt2:2<br>internal/x/z_test.go:1 |",
		"| RFC7519-AUD-FORMAT | formal/z3/model/standards.smt2:3 |  |  |",
		"| RFC7519-AUD-NA |  | formal/z3/model/standards.smt2:5 |  |",
		"| RFC7519-AUD-REJECT | formal/z3/model/standards.smt2:1 |  | formal/z3/checks/safety/aud.smt2:2<br>internal/x/z_test.go:1 |",
		"| RFC7519-ISS-ISSUER |  | formal/z3/model/standards.smt2:6 |  |",
	)
	if got := out.String(); got != want {
		t.Errorf("writeMap() =\n%s\nwant\n%s", got, want)
	}
}
