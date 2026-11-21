package request

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// 要素の前後から除く空白と、区切りの両側に許さない空白。Unicode の空白類は含めず、Lambda が文字種で判定する。
const whitespace = " \t\r\n"

// Parse は、repositories と permissions の入力のリストを検証し、発行要求を作る。
// 両方の入力のすべての誤りを 1 つの error にまとめ、通信の前に利用者がまとめて直せるようにする。
func Parse(repositories, permissions string) (TokenRequest, error) {
	repositoryEntries, repositoryErr := parseRepositories(splitElements(repositories))
	permissionEntries, permissionErr := parsePermissions(splitElements(permissions))
	if err := errors.Join(repositoryErr, permissionErr); err != nil {
		return TokenRequest{}, err
	}
	return TokenRequest{repositories: repositoryEntries, permissions: permissionEntries}, nil
}

// splitElements は、空の入力を空の要素 1 つとして扱う。空の入力を黙って通さず、空の要素の誤りにするため。
func splitElements(list string) []string {
	elements := strings.Split(list, ",")
	for i, element := range elements {
		// strings.TrimSpace は Unicode の空白類も除くため使わない
		elements[i] = strings.Trim(element, whitespace)
	}
	return elements
}

func parseRepositories(elements []string) ([]string, error) {
	var errs []error
	var repositories []string
	for i, element := range elements {
		const label = "repositories"
		if err := checkElement(label, i, element, "owner/repo", "/"); err != nil {
			errs = append(errs, err)
			continue
		}
		duplicate := false
		for _, earlier := range repositories {
			if equalFoldASCII(earlier, element) {
				errs = append(errs, fmt.Errorf("%s: %s, %s are duplicates (compared case-insensitively)", label, quote(earlier), quote(element)))
				duplicate = true
				break
			}
		}
		if !duplicate {
			repositories = append(repositories, element)
		}
	}
	return repositories, errors.Join(errs...)
}

func parsePermissions(elements []string) ([]permissionEntry, error) {
	var errs []error
	var entries []permissionEntry
	var originals []string
	for i, element := range elements {
		const label = "permissions"
		if err := checkElement(label, i, element, "name:level", ":"); err != nil {
			errs = append(errs, err)
			continue
		}
		name, level, _ := splitPair(element, ":")
		duplicate := false
		for j, earlier := range entries {
			if earlier.name == name {
				errs = append(errs, fmt.Errorf("%s: %s, %s have the same permission name", label, quote(originals[j]), quote(element)))
				duplicate = true
				break
			}
		}
		if !duplicate {
			entries = append(entries, permissionEntry{name: name, level: level})
			originals = append(originals, element)
		}
	}
	return entries, errors.Join(errs...)
}

// checkElement は、1 つの要素の空、UTF-8、形の誤りを、この順に最初の 1 つだけ返す。index は 0 始まりで、文言には 1 始まりで示す。
func checkElement(label string, index int, element, form, separator string) error {
	switch {
	case element == "":
		return fmt.Errorf("%s: element %d is empty", label, index+1)
	case !utf8.ValidString(element):
		return fmt.Errorf("%s: %s is not valid UTF-8", label, quote(element))
	}
	if _, _, ok := splitPair(element, separator); !ok {
		return fmt.Errorf("%s: %s is not in the form %s", label, quote(element), form)
	}
	return nil
}

// splitPair は、separator がちょうど 1 つで、両側が空でなく、区切りの前後に空白がない要素だけを分ける。
func splitPair(element, separator string) (left, right string, ok bool) {
	if strings.Count(element, separator) != 1 {
		return "", "", false
	}
	left, right, _ = strings.Cut(element, separator)
	if left == "" || right == "" {
		return "", "", false
	}
	if strings.ContainsRune(whitespace, rune(left[len(left)-1])) || strings.ContainsRune(whitespace, rune(right[0])) {
		return "", "", false
	}
	return left, right, true
}

// equalFoldASCII は、ASCII の範囲だけ大文字小文字を同一視する。
// strings.EqualFold は Unicode の fold をするため、Lambda の判定より広く重複と見なしてしまう。
func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range len(a) {
		if lowerASCII(a[i]) != lowerASCII(b[i]) {
			return false
		}
	}
	return true
}

func lowerASCII(b byte) byte {
	if 'A' <= b && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}

// quote は、不正な UTF-8 の要素も失敗せずに示すため、JSON ではなく strconv.Quote で引用する。
func quote(s string) string {
	return strconv.Quote(s)
}
