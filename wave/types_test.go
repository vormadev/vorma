package wave

import (
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave/internal/wavefilemap"
	"github.com/vormadev/vorma/wave/waveartifacts"
	"github.com/vormadev/vorma/wave/waveenv"
)

func TestApplyWaveFileOutputPrefixUsesRepositoryNamespacePrefix(t *testing.T) {
	if got := waveartifacts.ApplyWaveFileOutputPrefix("normal_deadbeef.css"); got != waveartifacts.HashedOutputPrefix+"normal_deadbeef.css" {
		t.Fatalf("apply wave file output prefix = %q", got)
	}
}

func TestApplyWaveOwnedFileOutputPrefixUsesRepositoryNamespacePrefix(
	t *testing.T,
) {
	got := waveartifacts.ApplyWaveOwnedFileOutputPrefix("normal_deadbeef.css")
	if got == "normal_deadbeef.css" {
		t.Fatalf("apply wave owned file output prefix = %q", got)
	}
	if !strings.HasSuffix(got, "normal_deadbeef.css") {
		t.Fatalf("apply wave owned file output prefix = %q", got)
	}
	if !strings.HasPrefix(
		got,
		waveartifacts.ApplyWaveOwnedFileOutputPrefix(""),
	) {
		t.Fatalf("apply wave owned file output prefix = %q", got)
	}
}

func TestWaveOwnedPublicArtifactPrefixComposition(t *testing.T) {
	composed := waveartifacts.ApplyWaveFileOutputPrefix(
		waveartifacts.ApplyWaveOwnedFileOutputPrefix("normal_deadbeef.css"),
	)
	if !strings.HasPrefix(composed, waveartifacts.HashedOutputPrefix) {
		t.Fatalf("wave owned public artifact composition = %q", composed)
	}
	if !strings.HasSuffix(composed, "normal_deadbeef.css") {
		t.Fatalf("wave owned public artifact composition = %q", composed)
	}
	if composed == waveartifacts.ApplyWaveFileOutputPrefix("normal_deadbeef.css") {
		t.Fatalf("wave owned public artifact composition = %q", composed)
	}
}

func TestFileMapLookupMappedAndMiss(t *testing.T) {
	fileMap := wavefilemap.FileMap{
		"logo.txt": {
			DistName: testHashedOutputRelativePath("logo.hash.txt"),
		},
	}

	url, found := fileMap.Lookup("/logo.txt", "/assets/")
	if !found {
		t.Fatal("expected mapped asset to be found")
	}
	if url != testHashedOutputPublicURL("logo.hash.txt") {
		t.Fatalf("unexpected mapped URL: %q", url)
	}

	missing, missingFound := fileMap.Lookup("missing.txt", "/assets/")
	if missingFound {
		t.Fatal("expected missing asset lookup to report not found")
	}
	if missing != "" {
		t.Fatalf("expected empty URL for missing lookup, got %q", missing)
	}
}

func TestResolvePublicURLFromReferencedPath(t *testing.T) {
	testCases := []struct {
		name                string
		publicPathPrefix    string
		referencedPath      string
		expectedResolvedURL string
	}{
		{
			name:                "joins normalized referenced path under prefix",
			publicPathPrefix:    "/assets/",
			referencedPath:      testHashedOutputRelativePath("styles.hash.css"),
			expectedResolvedURL: testHashedOutputPublicURL("styles.hash.css"),
		},
		{
			name:                "trims and normalizes traversal path under prefix",
			publicPathPrefix:    "/assets/",
			referencedPath:      " ../outside.css \n",
			expectedResolvedURL: "/assets/outside.css",
		},
		{
			name:                "root prefix remains rooted",
			publicPathPrefix:    "/",
			referencedPath:      "/styles/site.css",
			expectedResolvedURL: "/styles/site.css",
		},
		{
			name:                "empty path resolves to empty URL",
			publicPathPrefix:    "/assets/",
			referencedPath:      "",
			expectedResolvedURL: "",
		},
		{
			name:                "whitespace-only path resolves to empty URL",
			publicPathPrefix:    "/assets/",
			referencedPath:      " \n\t ",
			expectedResolvedURL: "",
		},
		{
			name:                "slash-only path resolves to empty URL",
			publicPathPrefix:    "/assets/",
			referencedPath:      "/",
			expectedResolvedURL: "",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			resolvedURL := waveenv.ResolveFromReferencedPath(
				testCase.publicPathPrefix,
				testCase.referencedPath,
			)
			if resolvedURL != testCase.expectedResolvedURL {
				t.Fatalf(
					"resolvePublicURLFromReferencedPath(%q, %q) = %q, want %q",
					testCase.publicPathPrefix,
					testCase.referencedPath,
					resolvedURL,
					testCase.expectedResolvedURL,
				)
			}
		})
	}
}
