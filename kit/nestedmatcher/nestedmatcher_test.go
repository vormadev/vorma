package nestedmatcher_test

import (
	"testing"

	"github.com/vormadev/vorma/kit/nestedmatcher"
)

func TestNewAndOptionGetters(t *testing.T) {
	m := nestedmatcher.New(&nestedmatcher.Options{
		DynamicParamPrefix:             '@',
		SplatSegmentIdentifier:         '#',
		ExplicitIndexSegmentIdentifier: "_index",
	})

	if got, want := m.DynamicParamPrefix(), rune('@'); got != want {
		t.Fatalf("DynamicParamPrefix() = %q, want %q", got, want)
	}
	if got, want := m.SplatSegmentIdentifier(), rune('#'); got != want {
		t.Fatalf("SplatSegmentIdentifier() = %q, want %q", got, want)
	}
	if got, want := m.ExplicitIndexSegmentIdentifier(), "_index"; got != want {
		t.Fatalf("ExplicitIndexSegmentIdentifier() = %q, want %q", got, want)
	}
}

func TestFindMatchesReturnsNestedResults(t *testing.T) {
	m := nestedmatcher.New(&nestedmatcher.Options{})
	m.RegisterPattern("")
	m.RegisterPattern("/users")
	m.RegisterPattern("/users/:id")

	results, found := m.FindMatches("/users/123")
	if !found {
		t.Fatal("expected nested match")
	}
	if results == nil {
		t.Fatal("results should not be nil")
	}
	if got, want := len(results.Matches), 3; got != want {
		t.Fatalf("len(Matches) = %d, want %d", got, want)
	}
	if got, want := results.Params["id"], "123"; got != want {
		t.Fatalf("Params[id] = %q, want %q", got, want)
	}
}
