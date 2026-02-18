package vormabuild

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vormadev/vorma/internal/vormaruntime"
)

func TestCollectBuildDiagnostics(t *testing.T) {
	t.Run("returns error when runtime is nil", func(t *testing.T) {
		snapshot, err := collectBuildDiagnostics(nil)
		if err == nil {
			t.Fatal("expected nil-runtime error")
		}
		if snapshot != nil {
			t.Fatalf("snapshot = %#v, expected nil snapshot on error", snapshot)
		}
		if err.Error() != "Vorma runtime is required" {
			t.Fatalf("error = %q, expected nil-runtime message", err)
		}
	})

	t.Run("returns error when config is nil", func(t *testing.T) {
		snapshot, err := collectBuildDiagnostics(&vormaruntime.Vorma{})
		if err == nil {
			t.Fatal("expected nil-config error")
		}
		if snapshot != nil {
			t.Fatalf("snapshot = %#v, expected nil snapshot on error", snapshot)
		}
		if err.Error() != "Vorma config is required" {
			t.Fatalf("error = %q, expected nil-config message", err)
		}
	})

	t.Run("wraps client route resolution errors", func(t *testing.T) {
		fixture := newBuildTestFixture(
			t,
			&buildTestFixtureOptions{config: newBuildDiagnosticsTestConfig()},
		)

		expectedErr := errors.New("client resolution failed")
		executor := newBuildDiagnosticsExecutorForTest(
			func(deps *buildDiagnosticsDependencies) {
				deps.resolveClientRouteDefinitionFiles = func(*vormaruntime.Vorma) ([]string, error) {
					return nil, expectedErr
				}
			},
		)

		snapshot, err := executor.collectBuildDiagnostics(fixture.app)
		if err == nil {
			t.Fatal("expected client-route resolution error")
		}
		if snapshot != nil {
			t.Fatalf("snapshot = %#v, expected nil snapshot on error", snapshot)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped client-route error", err)
		}
		if !strings.Contains(
			err.Error(),
			"resolve client route definition files",
		) {
			t.Fatalf("error = %q, expected client-route context", err)
		}
	})

	t.Run("wraps server route resolution errors", func(t *testing.T) {
		fixture := newBuildTestFixture(
			t,
			&buildTestFixtureOptions{config: newBuildDiagnosticsTestConfig()},
		)

		expectedErr := errors.New("server resolution failed")
		executor := newBuildDiagnosticsExecutorForTest(
			func(deps *buildDiagnosticsDependencies) {
				deps.resolveClientRouteDefinitionFiles = func(*vormaruntime.Vorma) ([]string, error) {
					return []string{"frontend/src/routes/client.routes.ts"}, nil
				}
				deps.resolveServerRouteDefinitionFiles = func(*vormaruntime.Vorma) ([]string, error) {
					return nil, expectedErr
				}
			},
		)

		snapshot, err := executor.collectBuildDiagnostics(fixture.app)
		if err == nil {
			t.Fatal("expected server-route resolution error")
		}
		if snapshot != nil {
			t.Fatalf("snapshot = %#v, expected nil snapshot on error", snapshot)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped server-route error", err)
		}
		if !strings.Contains(
			err.Error(),
			"resolve server route definition files",
		) {
			t.Fatalf("error = %q, expected server-route context", err)
		}
	})

	t.Run("collects deterministic snapshot values", func(t *testing.T) {
		fixture := newBuildTestFixture(
			t,
			&buildTestFixtureOptions{config: newBuildDiagnosticsTestConfig()},
		)
		app := fixture.app
		app.WithLock(func(l *vormaruntime.LockedVorma) {
			l.SetIsDev(true)
			l.SetBuildID("build-diagnostics-123")
			l.SetRouteManifestFile("route-manifest-diagnostics.json")
			l.SetPaths(map[string]*vormaruntime.Path{
				"/home": {
					OriginalPattern: "/home",
					SrcPath:         "frontend/src/routes/home.tsx",
				},
				"/about": {
					OriginalPattern: "/about",
					SrcPath:         "frontend/src/routes/about.tsx",
				},
			})
		})

		fixedNow := time.Date(
			2026,
			time.January,
			2,
			3,
			4,
			5,
			987654321,
			time.UTC,
		)
		clientRouteFiles := []string{
			"frontend/src/routes/home.vorma.routes.ts",
			"frontend/src/routes/about.vorma.routes.ts",
		}
		serverRouteFiles := []string{
			"backend/src/routes/home.go",
			"backend/src/routes/about.go",
		}
		executor := newBuildDiagnosticsExecutorForTest(
			func(deps *buildDiagnosticsDependencies) {
				deps.nowUTC = func() time.Time {
					return fixedNow
				}
				deps.resolveClientRouteDefinitionFiles = func(*vormaruntime.Vorma) ([]string, error) {
					return clientRouteFiles, nil
				}
				deps.resolveServerRouteDefinitionFiles = func(*vormaruntime.Vorma) ([]string, error) {
					return serverRouteFiles, nil
				}
				deps.discoveredRegistrarCacheKey = func(*vormaruntime.Vorma) string {
					return "diagnostics-cache-key"
				}
				deps.discoveredRegistrarCacheStats = func() (int, int) {
					return 4, 128
				}
			},
		)

		snapshot, err := executor.collectBuildDiagnostics(app)
		if err != nil {
			t.Fatalf("collectBuildDiagnostics returned error: %v", err)
		}
		if snapshot == nil {
			t.Fatal("expected snapshot")
		}

		if snapshot.GeneratedAtUTC != fixedNow.Format(time.RFC3339Nano) {
			t.Fatalf(
				"GeneratedAtUTC = %q, want %q",
				snapshot.GeneratedAtUTC,
				fixedNow.Format(time.RFC3339Nano),
			)
		}
		if snapshot.ConfigFile != filepath.ToSlash(
			filepath.Clean(app.Wave.ConfigFile()),
		) {
			t.Fatalf(
				"ConfigFile = %q, expected clean slashed config path",
				snapshot.ConfigFile,
			)
		}
		if snapshot.DistDir != filepath.ToSlash(
			filepath.Clean(app.Wave.DistDir()),
		) {
			t.Fatalf(
				"DistDir = %q, expected clean slashed dist path",
				snapshot.DistDir,
			)
		}
		if snapshot.StaticPrivateOut != filepath.ToSlash(
			filepath.Clean(app.Wave.StaticPrivateOutDir()),
		) {
			t.Fatalf(
				"StaticPrivateOut = %q, expected clean slashed static private path",
				snapshot.StaticPrivateOut,
			)
		}
		if snapshot.StaticPublicOut != filepath.ToSlash(
			filepath.Clean(app.Wave.StaticPublicOutDir()),
		) {
			t.Fatalf(
				"StaticPublicOut = %q, expected clean slashed static public path",
				snapshot.StaticPublicOut,
			)
		}
		if snapshot.MainBuildEntry != app.Config.MainBuildEntry {
			t.Fatalf(
				"MainBuildEntry = %q, want %q",
				snapshot.MainBuildEntry,
				app.Config.MainBuildEntry,
			)
		}
		if snapshot.ClientEntry != app.Config.ClientEntry {
			t.Fatalf(
				"ClientEntry = %q, want %q",
				snapshot.ClientEntry,
				app.Config.ClientEntry,
			)
		}
		if snapshot.UIVariant != app.Config.UIVariant {
			t.Fatalf(
				"UIVariant = %q, want %q",
				snapshot.UIVariant,
				app.Config.UIVariant,
			)
		}
		if snapshot.TSGenOutDir != app.Config.TSGenOutDir {
			t.Fatalf(
				"TSGenOutDir = %q, want %q",
				snapshot.TSGenOutDir,
				app.Config.TSGenOutDir,
			)
		}
		if !snapshot.IsDevMode {
			t.Fatal("expected IsDevMode to be true")
		}
		if snapshot.CurrentBuildID != "build-diagnostics-123" {
			t.Fatalf(
				"CurrentBuildID = %q, want %q",
				snapshot.CurrentBuildID,
				"build-diagnostics-123",
			)
		}
		if snapshot.CurrentRoutes != 2 {
			t.Fatalf("CurrentRoutes = %d, want 2", snapshot.CurrentRoutes)
		}
		if snapshot.CurrentManifest != "route-manifest-diagnostics.json" {
			t.Fatalf(
				"CurrentManifest = %q, want %q",
				snapshot.CurrentManifest,
				"route-manifest-diagnostics.json",
			)
		}
		if snapshot.CurrentClientOut != app.ClientEntryOut() {
			t.Fatalf(
				"CurrentClientOut = %q, want runtime client entry out %q",
				snapshot.CurrentClientOut,
				app.ClientEntryOut(),
			)
		}
		if snapshot.DiscoveredRegistrarCacheKey != "diagnostics-cache-key" {
			t.Fatalf(
				"DiscoveredRegistrarCacheKey = %q, want %q",
				snapshot.DiscoveredRegistrarCacheKey,
				"diagnostics-cache-key",
			)
		}
		if snapshot.DiscoveredRegistrarCacheEntryCount != 4 {
			t.Fatalf(
				"DiscoveredRegistrarCacheEntryCount = %d, want 4",
				snapshot.DiscoveredRegistrarCacheEntryCount,
			)
		}
		if snapshot.DiscoveredRegistrarCacheMaxEntries != 128 {
			t.Fatalf(
				"DiscoveredRegistrarCacheMaxEntries = %d, want 128",
				snapshot.DiscoveredRegistrarCacheMaxEntries,
			)
		}

		expectStringSlicesEqual(
			t,
			snapshot.ClientRouteDefinitionPatterns,
			[]string{
				"frontend/src/routes/*.client.ts",
				"frontend/src/routes/*.extra.client.ts",
			},
		)
		expectStringSlicesEqual(
			t,
			snapshot.ServerRouteDefinitionPatterns,
			[]string{
				"backend/src/routes/*.go",
				"backend/src/routes/admin/*.go",
			},
		)
		expectStringSlicesEqual(
			t,
			snapshot.ResolvedClientRouteFiles,
			clientRouteFiles,
		)
		expectStringSlicesEqual(
			t,
			snapshot.ResolvedServerRouteFiles,
			serverRouteFiles,
		)
	})
}

func TestPrintBuildDiagnostics(t *testing.T) {
	t.Run("wraps JSON marshal errors", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		expectedErr := errors.New("marshal failed")
		executor := newBuildDiagnosticsExecutorForTest(
			func(deps *buildDiagnosticsDependencies) {
				deps.resolveClientRouteDefinitionFiles = func(*vormaruntime.Vorma) ([]string, error) {
					return []string{}, nil
				}
				deps.resolveServerRouteDefinitionFiles = func(*vormaruntime.Vorma) ([]string, error) {
					return []string{}, nil
				}
				deps.marshalBuildDiagnosticsJSON = func(any) ([]byte, error) {
					return nil, expectedErr
				}
				deps.writeBuildDiagnosticsOutput = func([]byte) error {
					t.Fatal(
						"did not expect writeBuildDiagnosticsOutput when marshal fails",
					)
					return nil
				}
			},
		)

		err := executor.printBuildDiagnostics(fixture.app)
		if err == nil {
			t.Fatal("expected marshal error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped marshal error", err)
		}
		if !strings.Contains(err.Error(), "marshal build diagnostics JSON") {
			t.Fatalf("error = %q, expected marshal context", err)
		}
	})

	t.Run("wraps output write errors", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		expectedErr := errors.New("write failed")
		executor := newBuildDiagnosticsExecutorForTest(
			func(deps *buildDiagnosticsDependencies) {
				deps.resolveClientRouteDefinitionFiles = func(*vormaruntime.Vorma) ([]string, error) {
					return []string{}, nil
				}
				deps.resolveServerRouteDefinitionFiles = func(*vormaruntime.Vorma) ([]string, error) {
					return []string{}, nil
				}
				deps.marshalBuildDiagnosticsJSON = func(any) ([]byte, error) {
					return []byte(`{"ok":true}`), nil
				}
				deps.writeBuildDiagnosticsOutput = func([]byte) error {
					return expectedErr
				}
			},
		)

		err := executor.printBuildDiagnostics(fixture.app)
		if err == nil {
			t.Fatal("expected write error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped write error", err)
		}
		if !strings.Contains(err.Error(), "write build diagnostics output") {
			t.Fatalf("error = %q, expected write context", err)
		}
	})

	t.Run("emits JSON with required diagnostics fields", func(t *testing.T) {
		fixture := newBuildTestFixture(
			t,
			&buildTestFixtureOptions{config: newBuildDiagnosticsTestConfig()},
		)
		app := fixture.app
		app.WithLock(func(l *vormaruntime.LockedVorma) {
			l.SetBuildID("diagnostics-output-build")
			l.SetRouteManifestFile("diagnostics-output-manifest.json")
			l.SetPaths(map[string]*vormaruntime.Path{
				"/output": {
					OriginalPattern: "/output",
					SrcPath:         "frontend/src/routes/output.tsx",
				},
			})
		})

		fixedNow := time.Date(2026, time.March, 4, 5, 6, 7, 0, time.UTC)
		clientRouteFiles := []string{
			"frontend/src/routes/output.vorma.routes.ts",
		}
		serverRouteFiles := []string{"backend/src/routes/output.go"}

		var capturedOutput []byte
		executor := newBuildDiagnosticsExecutorForTest(
			func(deps *buildDiagnosticsDependencies) {
				deps.nowUTC = func() time.Time {
					return fixedNow
				}
				deps.resolveClientRouteDefinitionFiles = func(*vormaruntime.Vorma) ([]string, error) {
					return clientRouteFiles, nil
				}
				deps.resolveServerRouteDefinitionFiles = func(*vormaruntime.Vorma) ([]string, error) {
					return serverRouteFiles, nil
				}
				deps.discoveredRegistrarCacheKey = func(*vormaruntime.Vorma) string {
					return "diagnostics-output-cache-key"
				}
				deps.discoveredRegistrarCacheStats = func() (int, int) {
					return 1, 64
				}
				deps.marshalBuildDiagnosticsJSON = func(input any) ([]byte, error) {
					return json.Marshal(input)
				}
				deps.writeBuildDiagnosticsOutput = func(output []byte) error {
					capturedOutput = append([]byte(nil), output...)
					return nil
				}
			},
		)

		if err := executor.printBuildDiagnostics(app); err != nil {
			t.Fatalf("printBuildDiagnostics returned error: %v", err)
		}
		if len(capturedOutput) == 0 {
			t.Fatal("expected diagnostics output")
		}

		var output map[string]any
		if err := json.Unmarshal(capturedOutput, &output); err != nil {
			t.Fatalf("unmarshal diagnostics output: %v", err)
		}

		requiredFields := []string{
			"generatedAtUTC",
			"currentBuildID",
			"currentRoutes",
			"currentRouteManifest",
			"resolvedClientRouteFiles",
			"resolvedServerRouteFiles",
			"discoveredRegistrarCacheKey",
			"discoveredRegistrarCacheEntryCount",
			"discoveredRegistrarCacheMaxEntries",
		}
		for _, requiredField := range requiredFields {
			if _, ok := output[requiredField]; !ok {
				t.Fatalf(
					"diagnostics output missing required field %q",
					requiredField,
				)
			}
		}

		if output["generatedAtUTC"] != fixedNow.Format(time.RFC3339Nano) {
			t.Fatalf(
				"generatedAtUTC = %#v, want %q",
				output["generatedAtUTC"],
				fixedNow.Format(time.RFC3339Nano),
			)
		}
		if output["currentBuildID"] != "diagnostics-output-build" {
			t.Fatalf(
				"currentBuildID = %#v, want %q",
				output["currentBuildID"],
				"diagnostics-output-build",
			)
		}
		if output["currentRouteManifest"] != "diagnostics-output-manifest.json" {
			t.Fatalf(
				"currentRouteManifest = %#v, want %q",
				output["currentRouteManifest"],
				"diagnostics-output-manifest.json",
			)
		}
		if output["discoveredRegistrarCacheKey"] != "diagnostics-output-cache-key" {
			t.Fatalf(
				"discoveredRegistrarCacheKey = %#v, want %q",
				output["discoveredRegistrarCacheKey"],
				"diagnostics-output-cache-key",
			)
		}
		if output["discoveredRegistrarCacheEntryCount"] != float64(1) {
			t.Fatalf(
				"discoveredRegistrarCacheEntryCount = %#v, want %v",
				output["discoveredRegistrarCacheEntryCount"],
				1,
			)
		}
		if output["discoveredRegistrarCacheMaxEntries"] != float64(64) {
			t.Fatalf(
				"discoveredRegistrarCacheMaxEntries = %#v, want %v",
				output["discoveredRegistrarCacheMaxEntries"],
				64,
			)
		}
	})
}

func newBuildDiagnosticsExecutorForTest(
	override func(*buildDiagnosticsDependencies),
) buildDiagnosticsExecutor {
	dependencies := defaultBuildDiagnosticsDependencies()
	if override != nil {
		override(&dependencies)
	}
	return newBuildDiagnosticsExecutor(dependencies)
}

func newBuildDiagnosticsTestConfig() *vormaruntime.VormaConfig {
	return &vormaruntime.VormaConfig{
		MainBuildEntry:       "backend/cmd/build",
		UIVariant:            string(vormaruntime.UIVariantReact),
		HTMLTemplateLocation: "entry.go.html",
		ClientEntry:          "frontend/src/vorma.entry.tsx",
		ClientRouteDefinitionPatterns: []string{
			"frontend/src/routes/*.client.ts",
			"frontend/src/routes/*.extra.client.ts",
		},
		ServerRouteDefinitionPatterns: []string{
			"backend/src/routes/*.go",
			"backend/src/routes/admin/*.go",
		},
		TSGenOutDir:                "frontend/src/vorma.gen",
		BuildtimePublicURLFuncName: "waveBuildtimeURL",
	}
}

func expectStringSlicesEqual(t *testing.T, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf(
			"slice length mismatch: got %d, want %d (%#v vs %#v)",
			len(got),
			len(want),
			got,
			want,
		)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf(
				"slice mismatch at index %d: got %q, want %q",
				index,
				got[index],
				want[index],
			)
		}
	}
}
