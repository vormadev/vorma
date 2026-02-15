package jsonschema

import "testing"

func TestToOxfordList_UsesProvidedConjunction(t *testing.T) {
	got := toOxfordList([]string{"alpha", "beta", "gamma"}, "and")
	want := `"alpha", "beta", and "gamma"`
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestUniqueFrom_UsesAndForThreeItems(t *testing.T) {
	got := UniqueFrom("alpha", "beta", "gamma")
	want := `Must be unique from "alpha", "beta", and "gamma"`
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
