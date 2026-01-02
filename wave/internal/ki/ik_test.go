package ki

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/kit/safecache"
	"golang.org/x/sync/semaphore"
)

const testRootDir = "testdata"

// testEnv holds our testing environment
type testEnv struct {
	config *Config
}

// setupTestEnv creates a new test environment
func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	privateStaticSrcDirName := "private-static"
	publicStaticSrcDirName := "public-static"

	// Set up the source directory structure
	sourceDirs := []string{privateStaticSrcDirName, publicStaticSrcDirName}

	// Set up the dist directory structure
	distDirs := []string{
		"dist/static/assets/public",
		"dist/static/assets/private",
		"dist/static/internal",
	}

	for _, dir := range append(sourceDirs, distDirs...) {
		if err := os.MkdirAll(filepath.Join(testRootDir, dir), 0755); err != nil {
			t.Fatalf("Failed to create directory structure: %v", err)
		}
	}

	c := &Config{
		_uc: &UserConfig{
			Core: &UserConfigCore{
				StaticAssetDirs: StaticAssetDirs{
					Private: filepath.Join(testRootDir, privateStaticSrcDirName),
					Public:  filepath.Join(testRootDir, publicStaticSrcDirName),
				},
				CSSEntryFiles: CSSEntryFiles{
					NonCritical: filepath.Join(testRootDir, "main.css"),
					Critical:    filepath.Join(testRootDir, "critical.css"),
				},
				DistDir:          filepath.Join(testRootDir, "dist"),
				MainAppEntry:     "cmd/app/main.go",
				PublicPathPrefix: "/bob/",
			},
		},
		Logger: colorlog.New("ik_test"),
	}

	c.cleanSources = CleanSources{
		Dist:                filepath.Clean(c._uc.Core.DistDir),
		PrivateStatic:       filepath.Clean(c._uc.Core.StaticAssetDirs.Private),
		PublicStatic:        filepath.Clean(c._uc.Core.StaticAssetDirs.Public),
		CriticalCSSEntry:    filepath.Clean(c._uc.Core.CSSEntryFiles.Critical),
		NonCriticalCSSEntry: filepath.Clean(c._uc.Core.CSSEntryFiles.NonCritical),
	}

	c._dist = toDistLayout(c.cleanSources.Dist)

	// Initialize the fileSemaphore
	c.fileSemaphore = semaphore.NewWeighted(100)

	// Set up embedded FS
	c.DistStaticFS = fsutil.MustSub(os.DirFS(filepath.Join(testRootDir, "dist")), "static")

	// Initialize safecache
	c.runtime_cache = runtimeCache{
		base_fs:                 safecache.New(c.get_initial_base_fs, nil),
		base_dir_fs:             safecache.New(c.get_initial_base_dir_fs, nil),
		public_fs:               safecache.New(func() (fs.FS, error) { return c.getSubFSPublic() }, nil),
		private_fs:              safecache.New(func() (fs.FS, error) { return c.getSubFSPrivate() }, nil),
		stylesheet_link_el:      safecache.New(c.getInitialStyleSheetLinkElement, GetIsDev),
		stylesheet_url:          safecache.New(c.getInitialStyleSheetURL, GetIsDev),
		critical_css:            safecache.New(c.getInitialCriticalCSSStatus, GetIsDev),
		public_filemap_from_gob: safecache.New(c.getInitialPublicFileMapFromGobRuntime, nil),
		public_filemap_url:      safecache.New(c.getInitialPublicFileMapURL, GetIsDev),
		public_urls:             safecache.NewMap(c.getInitialPublicURL, publicURLsKeyMaker, nil),
	}

	// Initialize dev cache if needed
	c.dev.matchResults = safecache.NewMap(c.get_initial_match_results, c.match_results_key_maker, nil)

	// Set to production mode for testing
	os.Setenv(modeKey, "production")

	return &testEnv{config: c}
}

// teardownTestEnv cleans up the test environment
func teardownTestEnv(t *testing.T) {
	t.Helper()

	if err := os.RemoveAll(testRootDir); err != nil {
		t.Errorf("Failed to remove test directory: %v", err)
	}

	// Reset environment variables
	os.Unsetenv(modeKey)
}

// createTestFile creates a file with given content in the test environment
func (env *testEnv) createTestFile(t *testing.T, relativePath, content string) {
	t.Helper()

	fullPath := filepath.Join(testRootDir, relativePath)
	dir := filepath.Dir(fullPath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create directory %s: %v", dir, err)
	}

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write file %s: %v", fullPath, err)
	}
}

// resetEnv resets environment variables to a known state
func resetEnv() {
	os.Unsetenv(modeKey)
	os.Unsetenv(portKey)
	os.Unsetenv(portHasBeenSetKey)
	os.Unsetenv(refreshServerPortKey)
}

func TestMain(m *testing.M) {
	code := m.Run()
	os.RemoveAll(testRootDir)
	os.Exit(code)
}
