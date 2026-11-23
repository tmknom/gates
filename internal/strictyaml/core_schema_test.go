package strictyaml

import "testing"

func TestResolvesToNonString(t *testing.T) {
	tests := []struct {
		name  string
		plain []string
		want  bool
	}{
		{name: "null forms resolve to null", plain: []string{"", "~", "null", "Null", "NULL"}, want: true},
		{name: "boolean forms resolve to bool", plain: []string{"true", "True", "TRUE", "false", "False", "FALSE"}, want: true},
		{name: "decimal forms resolve to int", plain: []string{"0", "1", "010", "-1", "+1"}, want: true},
		{name: "octal form resolves to int", plain: []string{"0o10", "0o7"}, want: true},
		{name: "hexadecimal form resolves to int", plain: []string{"0x10", "0x3A", "0xff"}, want: true},
		{name: "number forms resolve to float", plain: []string{"1.0", ".5", "1.", "-0.0", "1e3", "+12e03", "-2E+05"}, want: true},
		{name: "infinity forms resolve to float", plain: []string{".inf", ".Inf", ".INF", "-.inf", "+.Inf"}, want: true},
		{name: "not-a-number forms resolve to float", plain: []string{".nan", ".NaN", ".NAN"}, want: true},
		// YAML 1.1 の真偽値と、YAML のライブラリが文字列以外に解決する表記は、Core Schema では文字列である
		{name: "YAML 1.1 booleans resolve to str", plain: []string{"yes", "no", "on", "off", "y", "n"}, want: false},
		{name: "forms the YAML library resolves to non-strings resolve to str", plain: []string{"1_000", "0b10", "2001-12-14", "1:20"}, want: false},
		{name: "forms close to the patterns resolve to str", plain: []string{"nULL", "tRUE", "0o8", "0x", "0xg", "1e", ".", "-", "1.2.3", "-.nan", "+.NaN"}, want: false},
		{name: "names and levels resolve to str", plain: []string{"read", "platform-infra", "deploy_tools.v2"}, want: false},
	}
	for _, tt := range tests {
		for _, plain := range tt.plain {
			t.Run(tt.name+" "+plain, func(t *testing.T) {
				if got := resolvesToNonString(plain); got != tt.want {
					t.Errorf("resolvesToNonString(%q) = %v, want %v", plain, got, tt.want)
				}
			})
		}
	}
}
