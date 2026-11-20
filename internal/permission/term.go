package permission

import "fmt"

// maxTermLength の根拠は Lambda Design の「権限名と水準の文字種」に記述する。
const maxTermLength = 64

// errMalformedTerm は、権限名か水準が文字種を満たさないことを表す。文言に入力の値を含めない。
var errMalformedTerm = fmt.Errorf("must be 1 to %d lowercase ASCII letters or '_'", maxTermLength)

func wellFormedTerm(s string) bool {
	if s == "" || len(s) > maxTermLength {
		return false
	}
	for i := range len(s) {
		if (s[i] < 'a' || 'z' < s[i]) && s[i] != '_' {
			return false
		}
	}
	return true
}
