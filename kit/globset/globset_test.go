package globset

import "testing"

func TestMatchIncludesSimpleSubtree(t *testing.T) {
	set := MustCompile([]string{"src/**"})

	if !set.Match("src/main.go") {
		t.Fatal("expected src/main.go to match")
	}
	if set.Match("lib/main.go") {
		t.Fatal("did not expect lib/main.go to match")
	}
}

func TestMatchPatternWithoutSlashMatchesAnywhere(t *testing.T) {
	set := MustCompile([]string{"*.go"})

	if !set.Match("main.go") {
		t.Fatal("expected main.go to match")
	}
	if !set.Match("src/main.go") {
		t.Fatal("expected src/main.go to match")
	}
	if set.Match("src/main.ts") {
		t.Fatal("did not expect src/main.ts to match")
	}
}

func TestMatchLeadingSlashAnchorsToRoot(t *testing.T) {
	set := MustCompile([]string{"/*.go"})

	if !set.Match("main.go") {
		t.Fatal("expected main.go to match")
	}
	if set.Match("src/main.go") {
		t.Fatal("did not expect src/main.go to match")
	}
}

func TestMatchTrailingSlashOnlyMatchesDirectories(t *testing.T) {
	set := MustCompile([]string{".", "!build/"})

	if set.Match("build/") {
		t.Fatal("did not expect build/ to match")
	}
	if set.Match("build/out.go") {
		t.Fatal("did not expect build/out.go to match")
	}
	if !set.Match("build") {
		t.Fatal("expected file named build to stay matched")
	}
}

func TestMatchLastRuleWins(t *testing.T) {
	set := MustCompile([]string{
		"src/**",
		"!src/vendor/**",
		"src/vendor/keep.go",
	})

	if !set.Match("src/main.go") {
		t.Fatal("expected src/main.go to match")
	}
	if set.Match("src/vendor/drop.go") {
		t.Fatal("did not expect src/vendor/drop.go to match")
	}
	if !set.Match("src/vendor/keep.go") {
		t.Fatal("expected src/vendor/keep.go to match")
	}
}

func TestMatchDotSlashAnchorsToRoot(t *testing.T) {
	set := MustCompile([]string{"./build"})

	if !set.Match("build") {
		t.Fatal("expected build to match")
	}
	if set.Match("src/build") {
		t.Fatal("did not expect src/build to match")
	}
}

func TestRuleHasSegmentMatchesLiteralAndGlobSegments(t *testing.T) {
	rule, ok, err := Parse("**/node_modules/**")
	if err != nil || !ok {
		t.Fatal("expected parse to succeed")
	}
	if !rule.HasSegment("node_modules") {
		t.Fatal("expected node_modules segment to be found")
	}

	rule, ok, err = Parse("*modules/**")
	if err != nil || !ok {
		t.Fatal("expected parse to succeed")
	}
	if !rule.HasSegment("node_modules") {
		t.Fatal("expected segment glob to match node_modules")
	}
}

func TestMatchNormalizesCandidatePaths(t *testing.T) {
	set := MustCompile([]string{"src/**", "!src/vendor/**"})

	if !set.Match("./src//main.go") {
		t.Fatal("expected normalized src path to match")
	}
	if set.Match("./src/vendor/./dep.go") {
		t.Fatal("did not expect normalized vendor path to match")
	}
}
