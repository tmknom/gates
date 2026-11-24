package github_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/tmknom/gates/internal/github"
)

// ポリシー読み取りトークンの値を取り出せるのは GitHub Client だけで、package の外から値を読む公開メソッドを持たないことを確かめる。
//
//trace:checks AC-ISSUANCE-011
func TestPolicyReadToken_HasNoPublicMethodToExposeValue(t *testing.T) {
	var got []string
	for method := range reflect.TypeFor[github.PolicyReadToken]().Methods() {
		got = append(got, method.Name)
	}
	if want := []string{"Format", "IsZero", "LogValue", "MarshalJSON"}; !slices.Equal(got, want) {
		t.Errorf("methods of PolicyReadToken = %q, want %q", got, want)
	}
}
