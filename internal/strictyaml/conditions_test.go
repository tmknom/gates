package strictyaml

import (
	"slices"
	"testing"
)

func TestConditions_Add(t *testing.T) {
	var c Conditions
	c.Add("b")
	c.Add("a %d", 1)
	c.Add("b")
	if want := []string{"b", "a 1"}; !slices.Equal(c.List(), want) {
		t.Errorf("List() = %q, want %q", c.List(), want)
	}
}
