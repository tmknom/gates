package github

import "testing"

func TestAppJWT_ExposeReturnsCompactSerialization(t *testing.T) {
	const compact = "header.payload.signature"
	if got := NewAppJWT(compact).expose(); got != compact {
		t.Errorf("expose() = %q, want %q", got, compact)
	}
}
