package wave

import (
	"path"
	"strings"
	"testing"

	"github.com/vormadev/vorma/kit/matcher"
	"github.com/vormadev/vorma/wave/internal/wavefilemap"
	"github.com/vormadev/vorma/wave/waveartifacts"
	"github.com/vormadev/vorma/wave/waveconfig"
)

func FuzzFileMapLookup(f *testing.F) {
	f.Add("logo.txt", "/assets/")
	f.Add("/logo.txt", "/")
	f.Add("nested/../logo.txt", waveartifacts.AssetsDirname)
	f.Add("missing.txt", "/assets/")
	f.Add("/assets/logo.txt", "/assets/")
	f.Add("0/logo.txt", "/0/")

	fm := wavefilemap.FileMap{
		"logo.txt": {
			DistName: waveartifacts.ApplyWaveFileOutputPrefix("logo.hash.txt"),
		},
	}

	f.Fuzz(func(t *testing.T, original string, prefix string) {
		url, found := fm.Lookup(original, prefix)

		normalizedOriginal := strings.TrimPrefix(path.Clean("/"+original), "/")
		normalizedPrefix := strings.Trim(path.Clean("/"+prefix), "/")
		originalReferencesMappedKey := normalizedOriginal == "logo.txt"
		if !originalReferencesMappedKey && normalizedPrefix != "" {
			normalizedPrefixWithTrailingSlash := normalizedPrefix + "/"
			if strings.HasPrefix(normalizedOriginal, normalizedPrefixWithTrailingSlash) {
				deprefixedOriginal := strings.TrimPrefix(
					normalizedOriginal,
					normalizedPrefixWithTrailingSlash,
				)
				originalReferencesMappedKey = deprefixedOriginal == "logo.txt"
			}
		}

		if originalReferencesMappedKey != found {
			t.Fatalf(
				"Lookup mismatch for original %q prefix %q: expected found=%v got %v",
				original,
				prefix,
				originalReferencesMappedKey,
				found,
			)
		}

		if !found {
			if url != "" {
				t.Fatalf("missing lookup must return empty URL, got %q", url)
			}
			return
		}

		if url == "" || !strings.HasPrefix(url, "/") {
			t.Fatalf("mapped lookup must return a leading-slash URL, got %q", url)
		}

		expected := matcher.EnsureLeadingSlash(
			path.Join(
				prefix,
				waveartifacts.ApplyWaveFileOutputPrefix("logo.hash.txt"),
			),
		)
		if url != expected {
			t.Fatalf("unexpected mapped URL: got %q want %q", url, expected)
		}
	})
}

func FuzzPublicPathPrefixNormalization(f *testing.F) {
	f.Add("")
	f.Add("/")
	f.Add(waveartifacts.AssetsDirname)
	f.Add("/assets")
	f.Add("assets/")
	f.Add("/assets/")

	f.Fuzz(func(t *testing.T, prefix string) {
		cfg := &waveconfig.ParsedConfig{Core: &waveconfig.CoreConfig{PublicPathPrefix: prefix}}
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
