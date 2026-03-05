package wave

import (
	"bytes"
	"github.com/vormadev/vorma/internal/wavetest"
	"os"
	"path/filepath"
	"testing"

	"github.com/vormadev/vorma/wave/internal/wavefilemap"
)

func TestRawConfigJSONIsDefensivelyCopied(t *testing.T) {
	fixture := newWaveTestFixture(t)
	t.Chdir(fixture.root)
	originalConfigJSON := fixture.configJSON(t)
	expected := append([]byte(nil), originalConfigJSON...)
	configPath := fixture.mustWriteConfigFile(t)

	w := New(Config{
		FS:         os.DirFS(fixture.root),
		ConfigPath: configPath,
		Logger:     newDiscardLoggerForWaveTests(),
	})

	originalConfigJSON[0] = 'x'
	if got := w.RawConfigJSON(); !bytes.Equal(got, expected) {
		t.Fatalf(
			"expected Wave to retain internal copy of config JSON, got %q",
			string(got),
		)
	}

	exposed := w.RawConfigJSON()
	exposed[1] = 'x'
	if got := w.RawConfigJSON(); !bytes.Equal(got, expected) {
		t.Fatalf(
			"expected RawConfigJSON to return defensive copy, got %q",
			string(got),
		)
	}
}

func TestPublicURLCachingDiffersByMode(t *testing.T) {
	t.Run("production caches first value", func(t *testing.T) {
		fixture := newWaveTestFixture(t)
		w := newWaveForTest(
			t,
			fixture,
			false,
			os.DirFS(fixture.pathInRoot(fixture.cfg.Dist().Static())),
		)

		first := w.PublicURL("logo.txt")
		if first != testHashedOutputPublicURL("logo.hash.txt") {
			t.Fatalf("unexpected initial public URL: %q", first)
		}

		mustWriteGob(t, fixture.pathInRoot(fixture.cfg.Dist().PublicFileMapGob()), wavefilemap.FileMap{
			"logo.txt": {
				DistName: testHashedOutputRelativePath("logo.changed.txt"),
			},
		})

		second := w.PublicURL("logo.txt")
		if second != first {
			t.Fatalf(
				"expected production mode to cache public URL, got first=%q second=%q",
				first,
				second,
			)
		}
	})

	t.Run("development recomputes value", func(t *testing.T) {
		fixture := newWaveTestFixture(t)
		w := newWaveForTest(t, fixture, true, nil)

		first := w.PublicURL("logo.txt")
		if first != testHashedOutputPublicURL("logo.hash.txt") {
			t.Fatalf("unexpected initial public URL: %q", first)
		}

		mustWriteGob(t, fixture.pathInRoot(fixture.cfg.Dist().PublicFileMapGob()), wavefilemap.FileMap{
			"logo.txt": {
				DistName: testHashedOutputRelativePath("logo.changed.txt"),
			},
		})

		second := w.PublicURL("logo.txt")
		if second != testHashedOutputPublicURL("logo.changed.txt") {
			t.Fatalf(
				"expected development mode to recompute public URL, got %q",
				second,
			)
		}
	})
}

func TestIsPublicAssetCachingDiffersByModeForRootPrefix(t *testing.T) {
	t.Run("production caches first file existence", func(t *testing.T) {
		fixture := newWaveTestFixture(t)
		wavetest.SetCorePublicPathPrefix(fixture.cfg, "/")
		w := newWaveForTest(
			t,
			fixture,
			false,
			os.DirFS(fixture.pathInRoot(fixture.cfg.Dist().Static())),
		)

		if !w.runtime.IsPublicAsset("/logo.txt") {
			t.Fatal(
				"expected /logo.txt to be an asset on first production lookup",
			)
		}
		if err := os.Remove(filepath.Join(fixture.pathInRoot(fixture.cfg.Dist().StaticPublic()), "logo.txt")); err != nil {
			t.Fatalf("failed to remove public file: %v", err)
		}
		if !w.runtime.IsPublicAsset("/logo.txt") {
			t.Fatal(
				"expected production mode to reuse cached file-existence result",
			)
		}
	})

	t.Run("development recomputes file existence", func(t *testing.T) {
		fixture := newWaveTestFixture(t)
		wavetest.SetCorePublicPathPrefix(fixture.cfg, "/")
		w := newWaveForTest(t, fixture, true, nil)

		if !w.runtime.IsPublicAsset("/logo.txt") {
			t.Fatal(
				"expected /logo.txt to be an asset on first development lookup",
			)
		}
		if err := os.Remove(filepath.Join(fixture.pathInRoot(fixture.cfg.Dist().StaticPublic()), "logo.txt")); err != nil {
			t.Fatalf("failed to remove public file: %v", err)
		}
		if w.runtime.IsPublicAsset("/logo.txt") {
			t.Fatal("expected development mode to recompute file existence")
		}
	})
}

func TestIsPublicAssetCachingDiffersByModeForConfiguredPrefix(t *testing.T) {
	t.Run("production caches first file existence", func(t *testing.T) {
		fixture := newWaveTestFixture(t)
		w := newWaveForTest(
			t,
			fixture,
			false,
			os.DirFS(fixture.pathInRoot(fixture.cfg.Dist().Static())),
		)

		if !w.runtime.IsPublicAsset("/assets/logo.txt") {
			t.Fatal(
				"expected /assets/logo.txt to be an asset on first production lookup",
			)
		}
		if err := os.Remove(filepath.Join(fixture.pathInRoot(fixture.cfg.Dist().StaticPublic()), "logo.txt")); err != nil {
			t.Fatalf("failed to remove public file: %v", err)
		}
		if !w.runtime.IsPublicAsset("/assets/logo.txt") {
			t.Fatal(
				"expected production mode to reuse cached prefixed file-existence result",
			)
		}
	})

	t.Run("development recomputes file existence", func(t *testing.T) {
		fixture := newWaveTestFixture(t)
		w := newWaveForTest(t, fixture, true, nil)

		if !w.runtime.IsPublicAsset("/assets/logo.txt") {
			t.Fatal(
				"expected /assets/logo.txt to be an asset on first development lookup",
			)
		}
		if err := os.Remove(filepath.Join(fixture.pathInRoot(fixture.cfg.Dist().StaticPublic()), "logo.txt")); err != nil {
			t.Fatalf("failed to remove public file: %v", err)
		}
		if w.runtime.IsPublicAsset("/assets/logo.txt") {
			t.Fatal(
				"expected development mode to recompute prefixed file existence",
			)
		}
	})
}

func TestIsPublicAssetProductionDoesNotCacheNegativeResults(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(
		t,
		fixture,
		false,
		os.DirFS(fixture.pathInRoot(fixture.cfg.Dist().Static())),
	)

	if w.runtime.IsPublicAsset("/assets/newly-created.txt") {
		t.Fatal("expected missing asset to return false before file exists")
	}

	mustWriteFile(
		t,
		filepath.Join(fixture.pathInRoot(fixture.cfg.Dist().StaticPublic()), "newly-created.txt"),
		"now-present",
	)

	if !w.runtime.IsPublicAsset("/assets/newly-created.txt") {
		t.Fatal(
			"expected production mode to recompute and detect newly created asset",
		)
	}
}

func TestCriticalCSSCachingDiffersByMode(t *testing.T) {
	t.Run("production caches first critical css payload", func(t *testing.T) {
		fixture := newWaveTestFixture(t)
		w := newWaveForTest(
			t,
			fixture,
			false,
			os.DirFS(fixture.pathInRoot(fixture.cfg.Dist().Static())),
		)

		firstCSS := string(w.CriticalCSS())
		if firstCSS != "body{color:red;}" {
			t.Fatalf(
				"unexpected initial critical CSS state: css=%q",
				firstCSS,
			)
		}

		mustWriteFile(t, fixture.pathInRoot(fixture.cfg.Dist().CriticalCSS()), "body{color:green;}")

		secondCSS := string(w.CriticalCSS())
		if secondCSS != firstCSS {
			t.Fatalf(
				"expected production mode to keep cached critical CSS, got css %q -> %q",
				firstCSS,
				secondCSS,
			)
		}
	})

	t.Run("development recomputes critical css payload", func(t *testing.T) {
		fixture := newWaveTestFixture(t)
		w := newWaveForTest(t, fixture, true, nil)

		firstCSS := string(w.CriticalCSS())
		if firstCSS != "body{color:red;}" {
			t.Fatalf(
				"unexpected initial critical CSS state: css=%q",
				firstCSS,
			)
		}

		mustWriteFile(t, fixture.pathInRoot(fixture.cfg.Dist().CriticalCSS()), "body{color:green;}")

		secondCSS := string(w.CriticalCSS())
		if secondCSS != "body{color:green;}" {
			t.Fatalf(
				"expected development mode to recompute critical CSS, got %q",
				secondCSS,
			)
		}
	})
}

func TestStylesheetURLCachingDiffersByMode(t *testing.T) {
	t.Run("production caches first stylesheet URL", func(t *testing.T) {
		fixture := newWaveTestFixture(t)
		w := newWaveForTest(
			t,
			fixture,
			false,
			os.DirFS(fixture.pathInRoot(fixture.cfg.Dist().Static())),
		)

		first := w.runtime.StyleSheetURL()
		if first != testOwnedOutputPublicURL("normal_hash.css") {
			t.Fatalf("unexpected initial stylesheet URL: %q", first)
		}

		mustWriteFile(
			t,
			fixture.pathInRoot(fixture.cfg.Dist().NormalCSSRef()),
			testHashedOutputRelativePath("changed.css"),
		)

		second := w.runtime.StyleSheetURL()
		if second != first {
			t.Fatalf(
				"expected production mode to keep cached stylesheet URL, got first=%q second=%q",
				first,
				second,
			)
		}
	})

	t.Run("development recomputes stylesheet URL", func(t *testing.T) {
		fixture := newWaveTestFixture(t)
		w := newWaveForTest(t, fixture, true, nil)

		first := w.runtime.StyleSheetURL()
		if first != testOwnedOutputPublicURL("normal_hash.css") {
			t.Fatalf("unexpected initial stylesheet URL: %q", first)
		}

		mustWriteFile(
			t,
			fixture.pathInRoot(fixture.cfg.Dist().NormalCSSRef()),
			testHashedOutputRelativePath("changed.css"),
		)

		second := w.runtime.StyleSheetURL()
		if second != testHashedOutputPublicURL("changed.css") {
			t.Fatalf(
				"expected development mode to recompute stylesheet URL, got %q",
				second,
			)
		}
	})
}

func TestPublicFileMapURLCachingDiffersByMode(t *testing.T) {
	t.Run("production caches first file map URL", func(t *testing.T) {
		fixture := newWaveTestFixture(t)
		w := newWaveForTest(
			t,
			fixture,
			false,
			os.DirFS(fixture.pathInRoot(fixture.cfg.Dist().Static())),
		)

		first := w.runtime.PublicFileMapURL()
		if first != testOwnedOutputPublicURL("public_filemap_hash.js") {
			t.Fatalf("unexpected initial public file map URL: %q", first)
		}

		mustWriteFile(
			t,
			fixture.pathInRoot(fixture.cfg.Dist().PublicFileMapRef()),
			testHashedOutputRelativePath("changed_public_filemap.js"),
		)

		second := w.runtime.PublicFileMapURL()
		if second != first {
			t.Fatalf(
				"expected production mode to keep cached file map URL, got first=%q second=%q",
				first,
				second,
			)
		}
	})

	t.Run("development recomputes file map URL", func(t *testing.T) {
		fixture := newWaveTestFixture(t)
		w := newWaveForTest(t, fixture, true, nil)

		first := w.runtime.PublicFileMapURL()
		if first != testOwnedOutputPublicURL("public_filemap_hash.js") {
			t.Fatalf("unexpected initial public file map URL: %q", first)
		}

		mustWriteFile(
			t,
			fixture.pathInRoot(fixture.cfg.Dist().PublicFileMapRef()),
			testHashedOutputRelativePath("changed_public_filemap.js"),
		)

		second := w.runtime.PublicFileMapURL()
		if second != testHashedOutputPublicURL("changed_public_filemap.js") {
			t.Fatalf(
				"expected development mode to recompute file map URL, got %q",
				second,
			)
		}
	})
}
