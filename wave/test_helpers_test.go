package wave

import (
	"bytes"
	"encoding/gob"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/vormadev/vorma/internal/testpath"
	"github.com/vormadev/vorma/internal/wavetest"
	"github.com/vormadev/vorma/wave/internal/wavefilemap"
	"github.com/vormadev/vorma/wave/internal/waveruntimecore"
	"github.com/vormadev/vorma/wave/waveartifacts"
	"github.com/vormadev/vorma/wave/waveconfig"
	"github.com/vormadev/vorma/wave/waveenv"
)

type waveTestFixture struct {
	root string
	cfg  waveconfig.ParsedConfig
}

func (f *waveTestFixture) pathInRoot(pathRelativeToConfigFS string) string {
	if filepath.IsAbs(pathRelativeToConfigFS) {
		return filepath.Clean(pathRelativeToConfigFS)
	}
	return filepath.Join(f.root, pathRelativeToConfigFS)
}

func testHashedOutputRelativePath(fileName string) string {
	return waveartifacts.ApplyWaveFileOutputPrefix(fileName)
}

func testHashedOutputPublicURL(fileName string) string {
	return "/assets/" + testHashedOutputRelativePath(fileName)
}

func testOwnedOutputRelativePath(fileName string) string {
	return waveartifacts.ApplyWaveFileOutputPrefix(
		waveartifacts.ApplyWaveOwnedFileOutputPrefix(fileName),
	)
}

func testOwnedOutputPublicURL(fileName string) string {
	return "/assets/" + testOwnedOutputRelativePath(fileName)
}

func newWaveTestFixture(t *testing.T) *waveTestFixture {
	t.Helper()

	root := t.TempDir()
	pathInFixtureForParsedDistPath := func(distPath string) string {
		if filepath.IsAbs(distPath) {
			return filepath.Clean(distPath)
		}
		return filepath.Join(root, distPath)
	}
	cfg := wavetest.NewParsedConfigAtRoot(t, root)
	wavetest.SetStaticAssetDirectories(
		cfg,
		filepath.Join(root, "public-src"),
		filepath.Join(root, "private-src"),
	)
	wavetest.SetCSSEntryFiles(
		cfg,
		filepath.Join(root, "src", "critical.css"),
		filepath.Join(root, "src", "non_critical.css"),
	)
	wavetest.SetCorePublicPathPrefix(cfg, "/assets/")
	wavetest.SetWatchHealthcheckEndpoint(cfg, "/healthz")

	mustEnsureDir(t, pathInFixtureForParsedDistPath(cfg.Dist().StaticPublic()))
	mustEnsureDir(t, pathInFixtureForParsedDistPath(cfg.Dist().StaticPrivate()))
	mustEnsureDir(t, pathInFixtureForParsedDistPath(cfg.Dist().Internal()))

	mustWriteFile(
		t,
		filepath.Join(
			pathInFixtureForParsedDistPath(cfg.Dist().StaticPublic()),
			"logo.txt",
		),
		"logo",
	)
	mustWriteFile(
		t,
		filepath.Join(
			pathInFixtureForParsedDistPath(cfg.Dist().StaticPublic()),
			"favicon.ico",
		),
		"ico",
	)
	mustWriteFile(
		t,
		filepath.Join(
			pathInFixtureForParsedDistPath(cfg.Dist().StaticPublic()),
			waveartifacts.ApplyWaveFileOutputPrefix("logo.hash.txt"),
		),
		"hashed-logo",
	)
	mustWriteFile(
		t,
		filepath.Join(
			pathInFixtureForParsedDistPath(cfg.Dist().StaticPublic()),
			waveartifacts.ApplyWaveFileOutputPrefix("favicon.hash.ico"),
		),
		"hashed-ico",
	)
	mustWriteFile(
		t,
		filepath.Join(
			pathInFixtureForParsedDistPath(cfg.Dist().StaticPublic()),
			testOwnedOutputRelativePath("normal_hash.css"),
		),
		"body{color:blue;}",
	)
	mustWriteFile(
		t,
		filepath.Join(
			pathInFixtureForParsedDistPath(cfg.Dist().StaticPublic()),
			testOwnedOutputRelativePath("public_filemap_hash.js"),
		),
		"export const wavePublicFileMap = {}",
	)
	mustWriteFile(
		t,
		filepath.Join(
			pathInFixtureForParsedDistPath(cfg.Dist().StaticPrivate()),
			"template.html",
		),
		waveartifacts.PrivateDirname,
	)

	mustWriteFile(
		t,
		pathInFixtureForParsedDistPath(cfg.Dist().CriticalCSS()),
		"body{color:red;}",
	)
	mustWriteFile(
		t,
		pathInFixtureForParsedDistPath(cfg.Dist().NormalCSSRef()),
		testOwnedOutputRelativePath("normal_hash.css"),
	)
	mustWriteFile(
		t,
		pathInFixtureForParsedDistPath(cfg.Dist().PublicFileMapRef()),
		testOwnedOutputRelativePath("public_filemap_hash.js"),
	)

	mustWriteGob(
		t,
		pathInFixtureForParsedDistPath(cfg.Dist().PublicFileMapGob()),
		wavefilemap.FileMap{
			"logo.txt": {
				DistName:    testHashedOutputRelativePath("logo.hash.txt"),
				ContentHash: "logo-hash",
			},
			"favicon.ico": {
				DistName:    testHashedOutputRelativePath("favicon.hash.ico"),
				ContentHash: "favicon-hash",
			},
		},
	)

	return &waveTestFixture{root: root, cfg: cfg}
}

func (f *waveTestFixture) configJSON(t *testing.T) []byte {
	t.Helper()
	cfgForJSON := f.cfg.Clone()
	normalizeMachineAbsoluteFilesystemConfigPathsForWaveTestFixtureJSON(
		t,
		f.root,
		cfgForJSON,
	)
	data, err := wavetest.MarshalParsedConfigToRawJSON(cfgForJSON)
	if err != nil {
		t.Fatalf("failed to marshal config JSON: %v", err)
	}
	return data
}

func normalizeMachineAbsoluteFilesystemConfigPathsForWaveTestFixtureJSON(
	t *testing.T,
	root string,
	cfg waveconfig.ParsedConfig,
) {
	t.Helper()
	if cfg == nil || cfg.Core() == nil {
		return
	}

	wavetest.SetCoreMainAppEntry(cfg,
		pathRelativeToWaveTestFixtureRootForJSON(
			t,
			root,
			cfg.Core().MainAppEntry(),
		),
	)
	wavetest.SetCoreStaticAssetDirsPublic(cfg,
		pathRelativeToWaveTestFixtureRootForJSON(
			t,
			root,
			cfg.Core().StaticAssetDirsPublic(),
		),
	)
	wavetest.SetCoreStaticAssetDirsPrivate(cfg,
		pathRelativeToWaveTestFixtureRootForJSON(
			t,
			root,
			cfg.Core().StaticAssetDirsPrivate(),
		),
	)
	wavetest.SetCoreCriticalCSSEntryFile(cfg,
		pathRelativeToWaveTestFixtureRootForJSON(
			t,
			root,
			cfg.Core().CriticalCSSEntryFile(),
		),
	)
	wavetest.SetCoreNonCriticalCSSEntryFile(cfg,
		pathRelativeToWaveTestFixtureRootForJSON(
			t,
			root,
			cfg.Core().NonCriticalCSSEntryFile(),
		),
	)

	if cfg.Vite() != nil {
		wavetest.SetViteJSPackageManagerCmdDir(cfg,
			pathRelativeToWaveTestFixtureRootForJSON(
				t,
				root,
				cfg.Vite().JSPackageManagerCmdDir(),
			),
		)
		wavetest.SetViteConfigFile(cfg,
			pathRelativeToWaveTestFixtureRootForJSON(
				t,
				root,
				cfg.Vite().ViteConfigFile(),
			),
		)
	}

	if cfg.Watch() == nil {
		return
	}
	excludeDirectories := cfg.Watch().ExcludeDirs()
	for excludeDirectoryPatternIndex, excludeDirectoryPattern := range excludeDirectories {
		excludeDirectories[excludeDirectoryPatternIndex] = testpath.PathRelativeToCurrentWorkingDirectory(
			t,
			excludeDirectoryPattern,
		)
	}
	wavetest.SetWatchExcludeDirs(cfg, excludeDirectories)

	excludeFiles := cfg.Watch().ExcludeFiles()
	for excludeFilePatternIndex, excludeFilePattern := range excludeFiles {
		excludeFiles[excludeFilePatternIndex] = testpath.PathRelativeToCurrentWorkingDirectory(
			t,
			excludeFilePattern,
		)
	}
	wavetest.SetWatchExcludeFiles(cfg, excludeFiles)

	includePatterns := cfg.Watch().Include()
	for watchIncludeIndex, watchedFile := range includePatterns {
		includePatterns[watchIncludeIndex].Pattern = testpath.PathRelativeToCurrentWorkingDirectory(
			t,
			watchedFile.Pattern,
		)
		for hookIndex, onChangeHook := range watchedFile.OnChangeHooks {
			for excludedPatternIndex, excludedPattern := range onChangeHook.Exclude {
				includePatterns[watchIncludeIndex].OnChangeHooks[hookIndex].Exclude[excludedPatternIndex] = testpath.PathRelativeToCurrentWorkingDirectory(
					t,
					excludedPattern,
				)
			}
		}
	}
	wavetest.SetWatchInclude(cfg, includePatterns)
}

func pathRelativeToWaveTestFixtureRootForJSON(
	t *testing.T,
	root string,
	pathToNormalize string,
) string {
	return testpath.PathRelativeToRootForJSON(t, root, pathToNormalize)
}

func (f *waveTestFixture) mustWriteConfigFile(t *testing.T) string {
	t.Helper()
	configPath := filepath.Join(f.root, "wave.config.json")
	if writeErr := os.WriteFile(configPath, f.configJSON(t), 0o644); writeErr != nil {
		t.Fatalf("failed to write config file %s: %v", configPath, writeErr)
	}
	return "wave.config.json"
}

func mustEnsureDir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create dir %s: %v", dir, err)
	}
}

func mustWriteFile(t *testing.T, filePath string, content string) {
	t.Helper()
	mustEnsureDir(t, filepath.Dir(filePath))
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write file %s: %v", filePath, err)
	}
}

func mustWriteGob(t *testing.T, filePath string, data any) {
	t.Helper()
	mustEnsureDir(t, filepath.Dir(filePath))

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(data); err != nil {
		t.Fatalf("failed to encode gob for %s: %v", filePath, err)
	}

	if err := os.WriteFile(filePath, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("failed to write gob file %s: %v", filePath, err)
	}
}

func setWaveDevModeForTest(t *testing.T, isDev bool) {
	t.Helper()
	if isDev {
		t.Setenv(waveenv.EnvMode, waveenv.EnvModeDev)
		return
	}
	t.Setenv(waveenv.EnvMode, "production")
}

func resetPortCacheForTest() {
	defaultPortResolver = waveenv.NewResolver()
}

func readFileMapDetailsFromCacheForTest(
	w *Wave,
) *waveruntimecore.FileMapDetails {
	if w == nil || w.runtime == nil {
		return nil
	}
	return w.runtime.FileMapDetailsFromCache()
}

func mustReadFileFromFS(t *testing.T, filesystem fs.FS, filePath string) string {
	t.Helper()
	data, err := fs.ReadFile(filesystem, filePath)
	if err != nil {
		t.Fatalf("failed to read file %s: %v", filePath, err)
	}
	return string(data)
}
