package authn

import (
	jsonv1 "encoding/json"
	"slices"
	"testing"
	"unicode"
)

// GOJ-CLAIM-NAME-ALIAS が定める別名が、go-oidc のデコーダが 5 つの Claim 名として読む名前とも、isClaimAlias が拒否する名前とも一致することを確かめる。
// デコーダの照合は Go の Unicode の版で広がり得るため、単純な Case Folding で畳み込まれる文字はすべて候補に入れる。
//
//trace:checks AC-AUTHN-004
func TestClaimAliases_MatchDecoderAndRejection(t *testing.T) {
	aliases := claimAliases(func(r rune) []rune { return claimAliasLetters[r] })
	for _, name := range claimNameCandidates() {
		claimName, isAlias := aliases[name]
		if !isAlias && slices.Contains([]string{"iss", "aud", "exp", "iat", "nbf"}, name) {
			claimName = name
		}
		if got := decodedClaimName(t, name); got != claimName {
			t.Errorf("decoder under unicode %s reads %q as %q, want %q", unicode.Version, name, got, claimName)
		}
		if got := isClaimAlias(name); got != isAlias {
			t.Errorf("isClaimAlias(%q) = %v, want %v", name, got, isAlias)
		}
	}
}

// claimNameCandidates は、5 つの Claim 名と、その各文字を単純な Case Folding で同じ文字に畳み込まれる文字に置き換えた名前を返す。
// i には、トルコ語用の対応でだけ i と同じになる İ と、i に似るがどの Case Folding でも i にならない ı も充てる。
// 完全な Case Folding でだけ ss と同じになる iß も加える。
func claimNameCandidates() []string {
	letters := func(r rune) []rune {
		if r == 'i' {
			return append(simpleFoldOrbit(r), 'İ', 'ı')
		}
		return simpleFoldOrbit(r)
	}
	names := []string{"iss", "aud", "exp", "iat", "nbf", "iß"}
	for name := range claimAliases(letters) {
		names = append(names, name)
	}
	return names
}

// simpleFoldOrbit は、r と Unicode の単純な Case Folding で同じ文字に畳み込まれる文字を返す。
func simpleFoldOrbit(r rune) []rune {
	orbit := []rune{r}
	for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
		orbit = append(orbit, f)
	}
	return orbit
}

// decodedClaimName は、go-oidc が Claim を読む encoding/json (v1) が、name を 5 つの Claim 名のどれとして読むかを返す。どれとしても読まなければ空文字列を返す。
func decodedClaimName(t *testing.T, name string) string {
	t.Helper()
	var claims struct {
		Iss jsonv1.RawMessage `json:"iss"`
		Aud jsonv1.RawMessage `json:"aud"`
		Exp jsonv1.RawMessage `json:"exp"`
		Iat jsonv1.RawMessage `json:"iat"`
		Nbf jsonv1.RawMessage `json:"nbf"`
	}
	if err := jsonv1.Unmarshal([]byte(`{"`+name+`":0}`), &claims); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	for claimName, value := range map[string]jsonv1.RawMessage{"iss": claims.Iss, "aud": claims.Aud, "exp": claims.Exp, "iat": claims.Iat, "nbf": claims.Nbf} {
		if value != nil {
			return claimName
		}
	}
	return ""
}
