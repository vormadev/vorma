package matcher

import "testing"

func TestNewWithNilOptions_UsesDefaultSegmentIdentifiers(t *testing.T) {
	m := New(nil)

	if got := m.DynamicParamPrefix(); got != ':' {
		t.Fatalf("DynamicParamPrefix() = %q, want ':'", got)
	}
	if got := m.SplatSegmentIdentifier(); got != '*' {
		t.Fatalf("SplatSegmentIdentifier() = %q, want '*'", got)
	}
}

func TestMatcherWrapperAccessorsAndNormalization(t *testing.T) {
	m := New(&Options{
		DynamicParamPrefix:     '$',
		SplatSegmentIdentifier: '#',
		Quiet:                  true,
	})

	if got := m.DynamicParamPrefix(); got != '$' {
		t.Fatalf("DynamicParamPrefix() = %q, want '$'", got)
	}
	if got := m.SplatSegmentIdentifier(); got != '#' {
		t.Fatalf("SplatSegmentIdentifier() = %q, want '#'", got)
	}

	normalizedPattern := m.NormalizePattern("/users/$id/#")
	if normalizedPattern == nil {
		t.Fatal("NormalizePattern() returned nil")
	}
	if got, want := normalizedPattern.NormalizedPattern(), "/users/:id/*"; got != want {
		t.Fatalf(
			"NormalizePattern().NormalizedPattern() = %q, want %q",
			got,
			want,
		)
	}
}

func TestMatcherWrapperHelpers(t *testing.T) {
	if !HasLeadingSlash("/users") {
		t.Fatal("HasLeadingSlash(\"/users\") = false, want true")
	}
	if HasLeadingSlash("users") {
		t.Fatal("HasLeadingSlash(\"users\") = true, want false")
	}

	if !HasTrailingSlash("/users/") {
		t.Fatal("HasTrailingSlash(\"/users/\") = false, want true")
	}
	if HasTrailingSlash("/users") {
		t.Fatal("HasTrailingSlash(\"/users\") = true, want false")
	}

	if got, want := EnsureLeadingSlash("users"), "/users"; got != want {
		t.Fatalf("EnsureLeadingSlash(\"users\") = %q, want %q", got, want)
	}
	if got, want := EnsureTrailingSlash("/users"), "/users/"; got != want {
		t.Fatalf("EnsureTrailingSlash(\"/users\") = %q, want %q", got, want)
	}
	if got, want := EnsureLeadingAndTrailingSlash("users"), "/users/"; got != want {
		t.Fatalf(
			"EnsureLeadingAndTrailingSlash(\"users\") = %q, want %q",
			got,
			want,
		)
	}
	if got, want := StripLeadingSlash("/users"), "users"; got != want {
		t.Fatalf("StripLeadingSlash(\"/users\") = %q, want %q", got, want)
	}
	if got, want := StripTrailingSlash("/users/"), "/users"; got != want {
		t.Fatalf("StripTrailingSlash(\"/users/\") = %q, want %q", got, want)
	}

	m := New(nil)
	registeredPattern := m.NormalizePattern("/users/:id")
	if registeredPattern == nil {
		t.Fatal("NormalizePattern(\"/users/:id\") returned nil")
	}

	if got, want := JoinPatterns(registeredPattern, "posts"), "/users/:id/posts"; got != want {
		t.Fatalf("JoinPatterns(..., \"posts\") = %q, want %q", got, want)
	}
	if got, want := JoinPatterns(registeredPattern, "/posts"), "/users/:id/posts"; got != want {
		t.Fatalf("JoinPatterns(..., \"/posts\") = %q, want %q", got, want)
	}
}

func TestRegisterPatternAndFindBestMatch(t *testing.T) {
	m := New(nil)
	m.RegisterPattern("/users/:id")

	bestMatch, found := m.FindBestMatch("/users/42")
	if !found {
		t.Fatal("FindBestMatch(\"/users/42\") found = false, want true")
	}
	if bestMatch == nil {
		t.Fatal("FindBestMatch(\"/users/42\") returned nil match")
	}
	if got, want := bestMatch.NormalizedPattern(), "/users/:id"; got != want {
		t.Fatalf("bestMatch.NormalizedPattern() = %q, want %q", got, want)
	}
	if got, want := bestMatch.Params["id"], "42"; got != want {
		t.Fatalf("bestMatch.Params[\"id\"] = %q, want %q", got, want)
	}

	if match, found := m.FindBestMatch("/orders/42"); found || match != nil {
		t.Fatalf(
			"FindBestMatch(\"/orders/42\") = (%v, %v), want (nil, false)",
			match,
			found,
		)
	}
}
