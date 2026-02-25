package registraroverlay

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type overlayReplaceConfigForTest struct {
	Replace map[string]string `json:"Replace"`
}

func TestDiscoveredRouteRegistrarArtifactCache(t *testing.T) {
	t.Run("cache hit returns immutable clones", func(t *testing.T) {
		cache := NewDiscoveredRouteRegistrarArtifactCache(4)
		cache.Set(
			"cache-key",
			"fingerprint-one",
			[]SourceArtifact{
				{
					TargetFilePath: "backend/src/router/discovered_route_registrar_0.gen.go",
					SourceBytes:    []byte("first"),
				},
			},
		)

		firstReadArtifacts, firstReadHit := cache.Get(
			"cache-key",
			"fingerprint-one",
		)
		if !firstReadHit {
			t.Fatal("expected cache hit for matching fingerprint")
		}
		if len(firstReadArtifacts) != 1 {
			t.Fatalf(
				"first read artifacts len = %d, want 1",
				len(firstReadArtifacts),
			)
		}
		firstReadArtifacts[0].SourceBytes[0] = 'X'

		secondReadArtifacts, secondReadHit := cache.Get(
			"cache-key",
			"fingerprint-one",
		)
		if !secondReadHit {
			t.Fatal("expected second cache hit for matching fingerprint")
		}
		if string(secondReadArtifacts[0].SourceBytes) != "first" {
			t.Fatalf(
				"cached artifacts should remain immutable clones, got %q",
				string(secondReadArtifacts[0].SourceBytes),
			)
		}
	})

	t.Run(
		"mismatched fingerprint invalidates stale cache entry",
		func(t *testing.T) {
			cache := NewDiscoveredRouteRegistrarArtifactCache(4)
			cache.Set(
				"cache-key",
				"fingerprint-one",
				[]SourceArtifact{
					{TargetFilePath: "x.go", SourceBytes: []byte("first")},
				},
			)

			_, staleFingerprintHit := cache.Get("cache-key", "fingerprint-two")
			if staleFingerprintHit {
				t.Fatal("expected cache miss for mismatched fingerprint")
			}
			_, oldFingerprintHitAfterInvalidation := cache.Get(
				"cache-key",
				"fingerprint-one",
			)
			if oldFingerprintHitAfterInvalidation {
				t.Fatal("expected stale cache entry to be invalidated")
			}
		},
	)

	t.Run("evicts least recently used entry over capacity", func(t *testing.T) {
		cache := NewDiscoveredRouteRegistrarArtifactCache(2)
		cache.Set("cache-key-a", "fp-a", nil)
		cache.Set("cache-key-b", "fp-b", nil)

		_, entryAHit := cache.Get("cache-key-a", "fp-a")
		if !entryAHit {
			t.Fatal("expected entry A cache hit before eviction step")
		}

		cache.Set("cache-key-c", "fp-c", nil)

		_, entryAHitAfterEviction := cache.Get("cache-key-a", "fp-a")
		if !entryAHitAfterEviction {
			t.Fatal("expected entry A to remain after LRU eviction")
		}
		_, entryBHitAfterEviction := cache.Get("cache-key-b", "fp-b")
		if entryBHitAfterEviction {
			t.Fatal("expected least recently used entry B to be evicted")
		}
		_, entryCHitAfterEviction := cache.Get("cache-key-c", "fp-c")
		if !entryCHitAfterEviction {
			t.Fatal("expected entry C to be present after insertion")
		}
	})
}

func TestComputeDiscoveredRouteRegistrarDiscoveryFingerprint(t *testing.T) {
	rootDir := t.TempDir()
	packageDir := filepath.Join(rootDir, "backend", "src", "router")
	routeFilePath := filepath.Join(packageDir, "routes.go")
	helperFilePath := filepath.Join(packageDir, "helper.go")

	mustWriteFile(
		t,
		routeFilePath,
		"package router\nvar RoutePattern = \"/before\"\n",
	)
	mustWriteFile(
		t,
		helperFilePath,
		"package router\nfunc helper() string { return \"one\" }\n",
	)

	firstFingerprint, err := ComputeDiscoveredRouteRegistrarDiscoveryFingerprint(
		[]string{routeFilePath},
	)
	if err != nil {
		t.Fatalf(
			"ComputeDiscoveredRouteRegistrarDiscoveryFingerprint returned error: %v",
			err,
		)
	}

	mustWriteFile(
		t,
		helperFilePath,
		"package router\nfunc helper() string { return \"two\" }\n",
	)

	secondFingerprint, err := ComputeDiscoveredRouteRegistrarDiscoveryFingerprint(
		[]string{routeFilePath},
	)
	if err != nil {
		t.Fatalf(
			"ComputeDiscoveredRouteRegistrarDiscoveryFingerprint second run returned error: %v",
			err,
		)
	}
	if secondFingerprint == firstFingerprint {
		t.Fatalf(
			"fingerprint should change when package Go source changes: %q",
			secondFingerprint,
		)
	}
}

func TestWriteDiscoveredRouteRegistrarOverlay(t *testing.T) {
	t.Run("returns nil when no artifacts are provided", func(t *testing.T) {
		overlay, err := WriteDiscoveredRouteRegistrarOverlay(nil)
		if err != nil {
			t.Fatalf(
				"WriteDiscoveredRouteRegistrarOverlay returned error: %v",
				err,
			)
		}
		if overlay != nil {
			t.Fatal("expected nil overlay when no artifacts are provided")
		}
	})

	t.Run("writes overlay config and source artifacts", func(t *testing.T) {
		targetFilePath := filepath.ToSlash(
			filepath.Clean(
				filepath.Join(
					t.TempDir(),
					"backend",
					"src",
					"router",
					GeneratedFilename,
				),
			),
		)
		artifact := SourceArtifact{
			TargetFilePath: targetFilePath,
			SourceBytes: []byte(
				"package router\n\nfunc init() {}\n",
			),
		}

		overlay, err := WriteDiscoveredRouteRegistrarOverlay(
			[]SourceArtifact{artifact},
		)
		if err != nil {
			t.Fatalf(
				"WriteDiscoveredRouteRegistrarOverlay returned error: %v",
				err,
			)
		}
		if overlay == nil {
			t.Fatal("expected non-nil overlay for non-empty artifacts")
		}

		overlayConfigBytes, err := os.ReadFile(overlay.GoOverlayConfigPath())
		if err != nil {
			t.Fatalf(
				"read overlay config %q: %v",
				overlay.GoOverlayConfigPath(),
				err,
			)
		}
		var overlayConfig overlayReplaceConfigForTest
		if err := json.Unmarshal(overlayConfigBytes, &overlayConfig); err != nil {
			t.Fatalf("unmarshal overlay config: %v", err)
		}
		overlaySourcePath, hasReplacement := overlayConfig.Replace[targetFilePath]
		if !hasReplacement {
			t.Fatalf(
				"overlay replacement missing target file %q in %#v",
				targetFilePath,
				overlayConfig.Replace,
			)
		}

		overlaySourceBytes, err := os.ReadFile(overlaySourcePath)
		if err != nil {
			t.Fatalf("read overlay source file %q: %v", overlaySourcePath, err)
		}
		if !strings.Contains(
			string(overlaySourceBytes),
			"func init() {}",
		) {
			t.Fatalf(
				"overlay source = %q, expected generated source content",
				string(overlaySourceBytes),
			)
		}

		overlayDirectoryPath := filepath.Dir(overlay.GoOverlayConfigPath())
		if err := overlay.Cleanup(); err != nil {
			t.Fatalf("cleanup overlay: %v", err)
		}
		if _, err := os.Stat(overlayDirectoryPath); !os.IsNotExist(err) {
			t.Fatalf(
				"expected overlay temp directory removal, stat err = %v",
				err,
			)
		}
	})
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir parent for %q: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file %q: %v", path, err)
	}
}
