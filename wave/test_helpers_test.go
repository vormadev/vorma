package wave

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/vormadev/vorma/internal/testpath"
	"github.com/vormadev/vorma/wave/internal/wavefilemap"
	"github.com/vormadev/vorma/wave/internal/waveruntimecore"
	"github.com/vormadev/vorma/wave/waveartifacts"
	"github.com/vormadev/vorma/wave/waveconfig"
	"github.com/vormadev/vorma/wave/waveenv"
)

type waveTestFixture struct {
	root string
	cfg  *waveconfig.ParsedConfig
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
	cfg := &waveconfig.ParsedConfig{
		Core: &waveconfig.CoreConfig{
			MainAppEntry: "cmd/app",
			DistDir:      filepath.Join(root, "dist"),
			StaticAssetDirs: waveconfig.StaticAssetDirs{
				Public:  filepath.Join(root, "public-src"),
				Private: filepath.Join(root, "private-src"),
			},
			CSSEntryFiles: waveconfig.CSSEntryFiles{
				Critical:    "./src/critical.css",
				NonCritical: "./src/non_critical.css",
			},
			PublicPathPrefix: "/assets/",
		},
		Vite: &waveconfig.ViteConfig{DefaultPort: 5173},
		Watch: &waveconfig.WatchConfig{
			WatchRoot:           "./watch/root",
			HealthcheckEndpoint: "/healthz",
		},
	}
	cfg.Dist.Root = cfg.Core.DistDir

	mustEnsureDir(t, cfg.Dist.StaticPublic())
	mustEnsureDir(t, cfg.Dist.StaticPrivate())
	mustEnsureDir(t, cfg.Dist.Internal())

	mustWriteFile(t, filepath.Join(cfg.Dist.StaticPublic(), "logo.txt"), "logo")
	mustWriteFile(t, filepath.Join(cfg.Dist.StaticPublic(), "favicon.ico"), "ico")
	mustWriteFile(
		t,
		filepath.Join(
			cfg.Dist.StaticPublic(),
			waveartifacts.ApplyWaveFileOutputPrefix("logo.hash.txt"),
		),
		"hashed-logo",
	)
	mustWriteFile(
		t,
		filepath.Join(
			cfg.Dist.StaticPublic(),
			waveartifacts.ApplyWaveFileOutputPrefix("favicon.hash.ico"),
		),
		"hashed-ico",
	)
	mustWriteFile(
		t,
		filepath.Join(
			cfg.Dist.StaticPublic(),
			testOwnedOutputRelativePath("normal_hash.css"),
		),
		"body{color:blue;}",
	)
	mustWriteFile(
		t,
		filepath.Join(
			cfg.Dist.StaticPublic(),
			testOwnedOutputRelativePath("public_filemap_hash.js"),
		),
		"export const wavePublicFileMap = {}",
	)
	mustWriteFile(
		t,
		filepath.Join(cfg.Dist.StaticPrivate(), "template.html"),
		waveartifacts.PrivateDirname,
	)

	mustWriteFile(t, cfg.Dist.CriticalCSS(), "body{color:red;}")
	mustWriteFile(
		t,
		cfg.Dist.NormalCSSRef(),
		testOwnedOutputRelativePath("normal_hash.css"),
	)
	mustWriteFile(
		t,
		cfg.Dist.PublicFileMapRef(),
		testOwnedOutputRelativePath("public_filemap_hash.js"),
	)

	mustWriteGob(t, cfg.Dist.PublicFileMapGob(), wavefilemap.FileMap{
		"logo.txt": {
			DistName:    testHashedOutputRelativePath("logo.hash.txt"),
			ContentHash: "logo-hash",
		},
		"favicon.ico": {
			DistName:    testHashedOutputRelativePath("favicon.hash.ico"),
			ContentHash: "favicon-hash",
		},
	})

	return &waveTestFixture{root: root, cfg: cfg}
}

func (f *waveTestFixture) configJSON(t *testing.T) []byte {
	t.Helper()
	cfgForJSON := f.cfg.Clone()
	normalizeMachineAbsoluteFilesystemConfigPathsForWaveTestFixtureJSON(
		t,
		cfgForJSON,
	)
	data, err := json.Marshal(cfgForJSON)
	if err != nil {
		t.Fatalf("failed to marshal config JSON: %v", err)
	}
	return data
}

func normalizeMachineAbsoluteFilesystemConfigPathsForWaveTestFixtureJSON(
	t *testing.T,
	cfg *waveconfig.ParsedConfig,
) {
	t.Helper()
	if cfg == nil || cfg.Core == nil {
		return
	}

	cfg.Core.MainAppEntry = testpath.PathRelativeToCurrentWorkingDirectory(
		t,
		cfg.Core.MainAppEntry,
	)
	cfg.Core.DistDir = testpath.PathRelativeToCurrentWorkingDirectory(
		t,
		cfg.Core.DistDir,
	)
	cfg.Core.StaticAssetDirs.Public = testpath.PathRelativeToCurrentWorkingDirectory(
		t,
		cfg.Core.StaticAssetDirs.Public,
	)
	cfg.Core.StaticAssetDirs.Private = testpath.PathRelativeToCurrentWorkingDirectory(
		t,
		cfg.Core.StaticAssetDirs.Private,
	)
	cfg.Core.CSSEntryFiles.Critical = testpath.PathRelativeToCurrentWorkingDirectory(
		t,
		cfg.Core.CSSEntryFiles.Critical,
	)
	cfg.Core.CSSEntryFiles.NonCritical = testpath.PathRelativeToCurrentWorkingDirectory(
		t,
		cfg.Core.CSSEntryFiles.NonCritical,
	)

	if cfg.Vite != nil {
		cfg.Vite.JSPackageManagerCmdDir = testpath.PathRelativeToCurrentWorkingDirectory(
			t,
			cfg.Vite.JSPackageManagerCmdDir,
		)
		cfg.Vite.ViteConfigFile = testpath.PathRelativeToCurrentWorkingDirectory(
			t,
			cfg.Vite.ViteConfigFile,
		)
	}

	if cfg.Watch == nil {
		return
	}
	cfg.Watch.WatchRoot = testpath.PathRelativeToCurrentWorkingDirectory(
		t,
		cfg.Watch.WatchRoot,
	)
	for excludeDirectoryPatternIndex, excludeDirectoryPattern := range cfg.Watch.Exclude.Dirs {
		cfg.Watch.Exclude.Dirs[excludeDirectoryPatternIndex] = testpath.PathRelativeToCurrentWorkingDirectory(
			t,
			excludeDirectoryPattern,
		)
	}
	for excludeFilePatternIndex, excludeFilePattern := range cfg.Watch.Exclude.Files {
		cfg.Watch.Exclude.Files[excludeFilePatternIndex] = testpath.PathRelativeToCurrentWorkingDirectory(
			t,
			excludeFilePattern,
		)
	}
	for watchIncludeIndex, watchedFile := range cfg.Watch.Include {
		cfg.Watch.Include[watchIncludeIndex].Pattern = testpath.PathRelativeToCurrentWorkingDirectory(
			t,
			watchedFile.Pattern,
		)
		for hookIndex, onChangeHook := range watchedFile.OnChangeHooks {
			for excludedPatternIndex, excludedPattern := range onChangeHook.Exclude {
				cfg.Watch.Include[watchIncludeIndex].OnChangeHooks[hookIndex].Exclude[excludedPatternIndex] = testpath.PathRelativeToCurrentWorkingDirectory(
					t,
					excludedPattern,
				)
			}
		}
	}
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
