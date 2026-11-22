package config_test

import (
	"os"
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/config"
)

// tfvarsExamplePath は、運用者が写して使う Terraform の変数の例である。
// 例の保護対象の一覧を、写しを持たずに Lambda の読み取りへ渡し、例を Lambda が拒否する値に書き換えたときにテストが失敗するようにする。
const tfvarsExamplePath = "../../terraform/terraform.tfvars.example"

// 環境変数の値は、Terraform が変数の文字列をそのまま入れるため、保護対象を置く例のヒアドキュメントの中身を固定設定の読み込みに渡す。
func TestLoad_AcceptsCommentedProtectedRepositoriesOfTFVarsExample(t *testing.T) {
	document := heredocOfTFVarsExample(t, "# protected_repositories = <<-EOT", "# EOT", "#   ")
	if _, err := config.Load(lookupEnv(withEnv(map[string]string{"GATES_PROTECTED_REPOSITORIES": document}))); err != nil {
		t.Errorf("Load returned error: %v\ndocument:\n%s", err, document)
	}
}

// 例の既定の値は、保護対象を置かないデプロイの値（AWS Infrastructure Design の「Lambda Deployment and Operation」）である。
func TestTFVarsExample_DefaultProtectedRepositoriesIsEmptyList(t *testing.T) {
	got := heredocOfTFVarsExample(t, "protected_repositories = <<-EOT", "EOT", "  ")
	if want := "repositories: []\n"; got != want {
		t.Errorf("protected_repositories = %q, want %q", got, want)
	}
}

// heredocOfTFVarsExample は、start の行の次から end の行の前までを、各行から indent を除き、行ごとに改行を付けて返す。
// Terraform の <<- は全行に共通する最小の字下げを除くが、例は本文を indent だけ下げる形に限り、その形でない行があれば失敗させる。
// 最小の字下げを計算し直すと、例の形の崩れに気付けないため。
func heredocOfTFVarsExample(t *testing.T, start, end, indent string) string {
	t.Helper()
	b, err := os.ReadFile(tfvarsExamplePath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	lines := strings.Split(string(b), "\n")
	begin := -1
	for i, line := range lines {
		if line == start {
			begin = i + 1
			break
		}
	}
	if begin < 0 {
		t.Fatalf("%s has no line %q", tfvarsExamplePath, start)
	}
	var document strings.Builder
	for _, line := range lines[begin:] {
		if line == end {
			return document.String()
		}
		body, ok := strings.CutPrefix(line, indent)
		if !ok {
			t.Fatalf("line %q in the heredoc is not indented by %q", line, indent)
		}
		document.WriteString(body + "\n")
	}
	t.Fatalf("%s has no line %q after %q", tfvarsExamplePath, end, start)
	return ""
}
