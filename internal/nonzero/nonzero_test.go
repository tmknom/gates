package nonzero_test

import (
	"fmt"
	"testing"

	"github.com/tmknom/gates/internal/nonzero"
)

// constructed は、構築関数を経た値だけが value を持つ型の例である。
type constructed struct {
	value string
}

func (c constructed) IsZero() bool {
	return c.value == ""
}

func TestRequire_NonZeroValueReturns(t *testing.T) {
	nonzero.Require(constructed{value: "secret"})
}

func TestRequire_ZeroValuePanicsWithTypeNameOnly(t *testing.T) {
	defer func() {
		got := recover()
		if got == nil {
			t.Fatal("Require did not panic for the zero value")
		}
		want := "zero value of nonzero_test.constructed"
		if fmt.Sprint(got) != want {
			t.Errorf("panic value = %v, want %q", got, want)
		}
	}()

	nonzero.Require(constructed{})
}
