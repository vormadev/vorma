package wave

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRawConfigJSONIsDefensivelyCopied(t *testing.T) {
	fixture := newWaveTestFixture(t)
	originalConfigJSON := fixture.configJSON(t)
	expected := append([]byte(nil), originalConfigJSON...)

	w := New(Config{
		WaveConfigJSON: originalConfigJSON,
		Logger:         newDiscardLoggerForWaveTests(),
	})

	originalConfigJSON[0] = 'x'
	if got := w.RawConfigJSON(); !bytes.Equal(got, expected) {
		t.Fatalf("expected Wave to retain internal copy of config JSON, got %q", string(got))
	}

	exposed := w.RawConfigJSON()
	exposed[1] = 'x'
	if got := w.RawConfigJSON(); !bytes.Equal(got, expected) {
		t.Fatalf("expected RawConfigJSON to return defensive copy, got %q", string(got))
	}
}

func TestGetPublicURLCachingDiffersByMode(t *testing.T) {
	t.Run("production caches first value", func(t *testing.T) {
		fixture := newWaveTestFixture(t)
		w := newWaveForTest(t, fixture, false, os.DirFS(fixture.cfg.Dist.Static()))

		first := w.GetPublicURL("logo.txt")
		if first != "/assets/vorma_out/logo.hash.txt" {
			t.Fatalf("unexpected initial public URL: %q", first)
		}

		mustWriteGob(t, fixture.cfg.Dist.PublicFileMapGob(), FileMap{
			"logo.txt": {
				DistName: "vorma_out/logo.changed.txt",
			},
		})

		second := w.GetPublicURL("logo.txt")
		if second != first {
			t.Fatalf("expected production mode to cache public URL, got first=%q second=%q", first, second)
		}
	})

	t.Run("development recomputes value", func(t *testing.T) {
		fixture := newWaveTestFixture(t)
		w := newWaveForTest(t, fixture, true, nil)

		first := w.GetPublicURL("logo.txt")
		if first != "/assets/vorma_out/logo.hash.txt" {
			t.Fatalf("unexpected initial public URL: %q", first)
		}

		mustWriteGob(t, fixture.cfg.Dist.PublicFileMapGob(), FileMap{
			"logo.txt": {
				DistName: "vorma_out/logo.changed.txt",
			},
		})

		second := w.GetPublicURL("logo.txt")
		if second != "/assets/vorma_out/logo.changed.txt" {
			t.Fatalf("expected development mode to recompute public URL, got %q", second)
		}
	})
}

func TestIsPublicAssetCachingDiffersByModeForRootPrefix(t *testing.T) {
	t.Run("production caches first file existence", func(t *testing.T) {
		fixture := newWaveTestFixture(t)
		fixture.cfg.Core.PublicPathPrefix = "/"
		w := newWaveForTest(t, fixture, false, os.DirFS(fixture.cfg.Dist.Static()))

		if !w.IsPublicAsset("/logo.txt") {
			t.Fatal("expected /logo.txt to be an asset on first production lookup")
		}
		if err := os.Remove(filepath.Join(fixture.cfg.Dist.StaticPublic(), "logo.txt")); err != nil {
			t.Fatalf("failed to remove public file: %v", err)
		}
		if !w.IsPublicAsset("/logo.txt") {
			t.Fatal("expected production mode to reuse cached file-existence result")
		}
	})

	t.Run("development recomputes file existence", func(t *testing.T) {
		fixture := newWaveTestFixture(t)
		fixture.cfg.Core.PublicPathPrefix = "/"
		w := newWaveForTest(t, fixture, true, nil)

		if !w.IsPublicAsset("/logo.txt") {
			t.Fatal("expected /logo.txt to be an asset on first development lookup")
		}
		if err := os.Remove(filepath.Join(fixture.cfg.Dist.StaticPublic(), "logo.txt")); err != nil {
			t.Fatalf("failed to remove public file: %v", err)
		}
		if w.IsPublicAsset("/logo.txt") {
			t.Fatal("expected development mode to recompute file existence")
		}
	})
}

func TestCriticalCSSCachingDiffersByMode(t *testing.T) {
	t.Run("production caches first critical css payload", func(t *testing.T) {
		fixture := newWaveTestFixture(t)
		w := newWaveForTest(t, fixture, false, os.DirFS(fixture.cfg.Dist.Static()))

		firstCSS := string(w.GetCriticalCSS())
		firstHash := w.GetCriticalCSSStyleElementSha256Hash()
		if firstCSS != "body{color:red;}" || firstHash == "" {
			t.Fatalf("unexpected initial critical CSS state: css=%q hash=%q", firstCSS, firstHash)
		}

		mustWriteFile(t, fixture.cfg.Dist.CriticalCSS(), "body{color:green;}")

		secondCSS := string(w.GetCriticalCSS())
		secondHash := w.GetCriticalCSSStyleElementSha256Hash()
		if secondCSS != firstCSS || secondHash != firstHash {
			t.Fatalf(
				"expected production mode to keep cached critical CSS, got css %q -> %q and hash %q -> %q",
				firstCSS,
				secondCSS,
				firstHash,
				secondHash,
			)
		}
	})

	t.Run("development recomputes critical css payload", func(t *testing.T) {
		fixture := newWaveTestFixture(t)
		w := newWaveForTest(t, fixture, true, nil)

		firstCSS := string(w.GetCriticalCSS())
		firstHash := w.GetCriticalCSSStyleElementSha256Hash()
		if firstCSS != "body{color:red;}" || firstHash == "" {
			t.Fatalf("unexpected initial critical CSS state: css=%q hash=%q", firstCSS, firstHash)
		}

		mustWriteFile(t, fixture.cfg.Dist.CriticalCSS(), "body{color:green;}")

		secondCSS := string(w.GetCriticalCSS())
		secondHash := w.GetCriticalCSSStyleElementSha256Hash()
		if secondCSS != "body{color:green;}" {
			t.Fatalf("expected development mode to recompute critical CSS, got %q", secondCSS)
		}
		if secondHash == firstHash {
			t.Fatalf("expected development mode to recompute critical CSS hash, hash remained %q", secondHash)
		}
	})
}

func TestStylesheetURLCachingDiffersByMode(t *testing.T) {
	t.Run("production caches first stylesheet URL", func(t *testing.T) {
		fixture := newWaveTestFixture(t)
		w := newWaveForTest(t, fixture, false, os.DirFS(fixture.cfg.Dist.Static()))

		first := w.GetStyleSheetURL()
		if first != "/assets/vorma_out/vorma_internal_normal_hash.css" {
			t.Fatalf("unexpected initial stylesheet URL: %q", first)
		}

		mustWriteFile(t, fixture.cfg.Dist.NormalCSSRef(), "vorma_out/changed.css")

		second := w.GetStyleSheetURL()
		if second != first {
			t.Fatalf("expected production mode to keep cached stylesheet URL, got first=%q second=%q", first, second)
		}
	})

	t.Run("development recomputes stylesheet URL", func(t *testing.T) {
		fixture := newWaveTestFixture(t)
		w := newWaveForTest(t, fixture, true, nil)

		first := w.GetStyleSheetURL()
		if first != "/assets/vorma_out/vorma_internal_normal_hash.css" {
			t.Fatalf("unexpected initial stylesheet URL: %q", first)
		}

		mustWriteFile(t, fixture.cfg.Dist.NormalCSSRef(), "vorma_out/changed.css")

		second := w.GetStyleSheetURL()
		if second != "/assets/vorma_out/changed.css" {
			t.Fatalf("expected development mode to recompute stylesheet URL, got %q", second)
		}
	})
}

func TestPublicFileMapURLCachingDiffersByMode(t *testing.T) {
	t.Run("production caches first file map URL", func(t *testing.T) {
		fixture := newWaveTestFixture(t)
		w := newWaveForTest(t, fixture, false, os.DirFS(fixture.cfg.Dist.Static()))

		first := w.GetPublicFileMapURL()
		if first != "/assets/vorma_out/vorma_internal_public_filemap_hash.js" {
			t.Fatalf("unexpected initial public file map URL: %q", first)
		}

		mustWriteFile(t, fixture.cfg.Dist.PublicFileMapRef(), "vorma_out/changed_public_filemap.js")

		second := w.GetPublicFileMapURL()
		if second != first {
			t.Fatalf("expected production mode to keep cached file map URL, got first=%q second=%q", first, second)
		}
	})

	t.Run("development recomputes file map URL", func(t *testing.T) {
		fixture := newWaveTestFixture(t)
		w := newWaveForTest(t, fixture, true, nil)

		first := w.GetPublicFileMapURL()
		if first != "/assets/vorma_out/vorma_internal_public_filemap_hash.js" {
			t.Fatalf("unexpected initial public file map URL: %q", first)
		}

		mustWriteFile(t, fixture.cfg.Dist.PublicFileMapRef(), "vorma_out/changed_public_filemap.js")

		second := w.GetPublicFileMapURL()
		if second != "/assets/vorma_out/changed_public_filemap.js" {
			t.Fatalf("expected development mode to recompute file map URL, got %q", second)
		}
	})
}
