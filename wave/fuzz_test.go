package wave

import (
	"path"
	"strings"
	"testing"

	"github.com/vormadev/vorma/kit/matcher"
)

func FuzzFileMapLookup(f *testing.F) {
	f.Add("logo.txt", "/assets/")
	f.Add("/logo.txt", "/")
	f.Add("nested/../logo.txt", "assets")
	f.Add("missing.txt", "/assets/")

	fm := FileMap{
		"logo.txt": {
			DistName: "vorma_out/logo.hash.txt",
		},
	}

	f.Fuzz(func(t *testing.T, original string, prefix string) {
		url, found := fm.Lookup(original, prefix)
		if url == "" || !strings.HasPrefix(url, "/") {
			t.Fatalf("Lookup must always return a leading-slash URL, got %q", url)
		}

		normalizedOriginal := strings.TrimPrefix(path.Clean("/"+original), "/")
		if normalizedOriginal == "logo.txt" {
			if !found {
				t.Fatalf("expected cleaned original %q to match mapped key", original)
			}
			expected := matcher.EnsureLeadingSlash(path.Join(prefix, "vorma_out/logo.hash.txt"))
			if url != expected {
				t.Fatalf("unexpected mapped URL: got %q want %q", url, expected)
			}
			return
		}

		if found {
			t.Fatalf("unexpected found=true for unmatched original %q", original)
		}
	})
}

func FuzzPublicPathPrefixNormalization(f *testing.F) {
	f.Add("")
	f.Add("/")
	f.Add("assets")
	f.Add("/assets")
	f.Add("assets/")
	f.Add("/assets/")

	f.Fuzz(func(t *testing.T, prefix string) {
		cfg := &ParsedConfig{Core: &CoreConfig{PublicPathPrefix: prefix}}
		normalized := cfg.PublicPathPrefix()

		if normalized == "" {
			t.Fatal("PublicPathPrefix must never return empty string")
		}
		if !strings.HasPrefix(normalized, "/") {
			t.Fatalf("PublicPathPrefix must always start with '/', got %q", normalized)
		}
		if normalized != "/" && !strings.HasSuffix(normalized, "/") {
			t.Fatalf("non-root PublicPathPrefix must always end with '/', got %q", normalized)
		}
	})
}
