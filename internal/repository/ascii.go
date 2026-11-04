package repository

func isASCIIAlphanumeric(c byte) bool {
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9'
}

// EqualFoldASCII は、ASCII の大文字小文字だけを同一視する。
// strings.EqualFold は Kelvin 記号（U+212A）と k のように、ASCII 外の文字も ASCII の文字と同一視するため使わない。
// 名前の比較規則を、名前を含む値（オーナーの URL など）の比較でも同じにするため公開する。
func EqualFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range len(a) {
		if toLowerASCII(a[i]) != toLowerASCII(b[i]) {
			return false
		}
	}
	return true
}

func toLowerASCII(c byte) byte {
	if 'A' <= c && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}
