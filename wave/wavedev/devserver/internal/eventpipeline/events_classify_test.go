package eventpipeline_test

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/internal/wavetest"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/wavebuild/builder"
	"github.com/vormadev/vorma/wave/wavedev/devserver/internal/eventpipeline"
	"github.com/vormadev/vorma/wave/wavedev/internal/watch"
)

func TestClassifyEventWithWatcherAndBuilder_GoFileCanBeTreatedAsNonGo(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForEventClassificationTestsAtRoot(root)
	cfg.Watch.Include = []wave.WatchedFile{
		{
			Pattern:      "**/*.go",
			TreatAsNonGo: true,
			RestartApp:   true,
		},
	}

	watcherForTest, watcherCreateError := watch.NewWatcher(
		cfg,
		newDiscardLoggerForEventClassificationTests(),
	)
	if watcherCreateError != nil {
		t.Fatalf("NewWatcher returned error: %v", watcherCreateError)
	}
	defer watcherForTest.Close()

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForEventClassificationTests(),
	)
	defer builderForTest.Close()

	watcherEvent := fsnotify.Event{
		Name: filepath.Join(root, "pkg", "handler.go"),
		Op:   fsnotify.Write,
	}

	classifiedEvent := classifyEventWithWatcherAndBuilderForEventPipelineTests(
		watcherEvent,
		watcherForTest,
		builderForTest,
	)
	if classifiedEvent.FileType != eventpipeline.FileTypeOther {
		t.Fatalf(
			"expected eventpipeline.FileTypeOther, got %v",
			classifiedEvent.FileType,
		)
	}
	if classifiedEvent.WatchedFile == nil {
		t.Fatal("expected matched watched file, got nil")
	}
	if classifiedEvent.Ignored {
		t.Fatal("expected event to be processed, got ignored=true")
	}
}

func TestClassifyEventWithWatcherAndBuilder_UnmatchedOtherFilesAreIgnored(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForEventClassificationTestsAtRoot(root)

	watcherForTest, watcherCreateError := watch.NewWatcher(
		cfg,
		newDiscardLoggerForEventClassificationTests(),
	)
	if watcherCreateError != nil {
		t.Fatalf("NewWatcher returned error: %v", watcherCreateError)
	}
	defer watcherForTest.Close()

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForEventClassificationTests(),
	)
	defer builderForTest.Close()

	watcherEvent := fsnotify.Event{
		Name: filepath.Join(root, "README.md"),
		Op:   fsnotify.Write,
	}

	classifiedEvent := classifyEventWithWatcherAndBuilderForEventPipelineTests(
		watcherEvent,
		watcherForTest,
		builderForTest,
	)
	if classifiedEvent.FileType != eventpipeline.FileTypeOther {
		t.Fatalf(
			"expected eventpipeline.FileTypeOther, got %v",
			classifiedEvent.FileType,
		)
	}
	if !classifiedEvent.Ignored {
		t.Fatal("expected unmatched non-special file to be ignored")
	}
}

func TestIsConfigFileMatchesNormalizedPath(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "backend", "wave.config.json")

	cfg := newParsedConfigForEventClassificationTestsAtRoot(root)
	cfg.Core.ConfigLocation = configPath
	equivalentPath := filepath.Join(root, "backend", ".", "wave.config.json")
	if !isConfigFileForEventPipelineTests(cfg, equivalentPath) {
		t.Fatalf(
			"expected isConfigFileForEventPipelineTests(%q) to match config path %q",
			equivalentPath,
			configPath,
		)
	}

	otherPath := filepath.Join(root, "backend", "different.config.json")
	if isConfigFileForEventPipelineTests(cfg, otherPath) {
		t.Fatalf(
			"expected isConfigFileForEventPipelineTests(%q) to be false",
			otherPath,
		)
	}

	cfg.Core.ConfigLocation = ""
	if isConfigFileForEventPipelineTests(cfg, configPath) {
		t.Fatal(
			"expected isConfigFileForEventPipelineTests to be false when config file path is empty",
		)
	}
}

func TestNeedsHardReload(t *testing.T) {
	if eventpipeline.NeedsHardReload(nil) {
		t.Fatal("eventpipeline.NeedsHardReload(nil) should be false")
	}
	if !eventpipeline.NeedsHardReload(
		&wave.WatchedFile{RecompileGoBinary: true},
	) {
		t.Fatal("expected RecompileGoBinary=true to require hard reload")
	}
	if !eventpipeline.NeedsHardReload(&wave.WatchedFile{RestartApp: true}) {
		t.Fatal("expected RestartApp=true to require hard reload")
	}
	if eventpipeline.NeedsHardReload(&wave.WatchedFile{}) {
		t.Fatal("expected empty watched file to not require hard reload")
	}
}

func TestClassifyEventWithWatcherAndBuilder_EmptyPathIsIgnored(t *testing.T) {
	cfg := newParsedConfigForEventClassificationTestsAtRoot(t.TempDir())

	watcherForTest, watcherCreateError := watch.NewWatcher(
		cfg,
		newDiscardLoggerForEventClassificationTests(),
	)
	if watcherCreateError != nil {
		t.Fatalf("NewWatcher returned error: %v", watcherCreateError)
	}
	defer watcherForTest.Close()

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForEventClassificationTests(),
	)
	defer builderForTest.Close()

	classifiedEvent := classifyEventWithWatcherAndBuilderForEventPipelineTests(
		fsnotify.Event{Name: "", Op: fsnotify.Write},
		watcherForTest,
		builderForTest,
	)

	if !classifiedEvent.Ignored {
		t.Fatal("expected empty-path event to be ignored")
	}
}

func TestClassifyEventWithWatcherAndBuilder_PublicAndPrivateStaticFiles(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForEventClassificationTestsAtRoot(root)

	watcherForTest, watcherCreateError := watch.NewWatcher(
		cfg,
		newDiscardLoggerForEventClassificationTests(),
	)
	if watcherCreateError != nil {
		t.Fatalf("NewWatcher returned error: %v", watcherCreateError)
	}
	defer watcherForTest.Close()

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForEventClassificationTests(),
	)
	defer builderForTest.Close()

	publicFilePath := filepath.Join(
		cfg.Core.StaticAssetDirs.Public,
		"img",
		"logo.png",
	)
	privateFilePath := filepath.Join(
		cfg.Core.StaticAssetDirs.Private,
		"tpl",
		"home.html",
	)

	publicClassifiedEvent := classifyEventWithWatcherAndBuilderForEventPipelineTests(
		fsnotify.Event{Name: publicFilePath, Op: fsnotify.Write},
		watcherForTest,
		builderForTest,
	)
	if publicClassifiedEvent.FileType != eventpipeline.FileTypePublicStatic {
		t.Fatalf(
			"expected public static file type, got %v",
			publicClassifiedEvent.FileType,
		)
	}

	privateClassifiedEvent := classifyEventWithWatcherAndBuilderForEventPipelineTests(
		fsnotify.Event{Name: privateFilePath, Op: fsnotify.Write},
		watcherForTest,
		builderForTest,
	)
	if privateClassifiedEvent.FileType != eventpipeline.FileTypePrivateStatic {
		t.Fatalf(
			"expected private static file type, got %v",
			privateClassifiedEvent.FileType,
		)
	}
}

func TestClassifyEventWithWatcherAndBuilder_CriticalAndNormalCSSFiles(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForEventClassificationTestsAtRoot(root)

	watcherForTest, watcherCreateError := watch.NewWatcher(
		cfg,
		newDiscardLoggerForEventClassificationTests(),
	)
	if watcherCreateError != nil {
		t.Fatalf("NewWatcher returned error: %v", watcherCreateError)
	}
	defer watcherForTest.Close()

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForEventClassificationTests(),
	)
	defer builderForTest.Close()

	criticalCSSFilePath := filepath.Join(root, "styles", "critical.css")
	normalCSSFilePath := filepath.Join(root, "styles", "normal.css")
	sharedCSSFilePath := filepath.Join(root, "styles", "shared.css")
	wavetest.SetCSSEntryFiles(cfg, criticalCSSFilePath, normalCSSFilePath)
	if makeDirectoryError := os.MkdirAll(filepath.Dir(criticalCSSFilePath), 0o755); makeDirectoryError != nil {
		t.Fatalf("create css directory: %v", makeDirectoryError)
	}
	if writeCriticalError := os.WriteFile(
		criticalCSSFilePath,
		[]byte(`@import "./shared.css"; .critical { color: red; }`),
		0o644,
	); writeCriticalError != nil {
		t.Fatalf("write critical css: %v", writeCriticalError)
	}
	if writeNormalError := os.WriteFile(
		normalCSSFilePath,
		[]byte(`@import "./shared.css"; .normal { color: blue; }`),
		0o644,
	); writeNormalError != nil {
		t.Fatalf("write normal css: %v", writeNormalError)
	}
	if writeSharedError := os.WriteFile(
		sharedCSSFilePath,
		[]byte(`.shared { color: green; }`),
		0o644,
	); writeSharedError != nil {
		t.Fatalf("write shared css: %v", writeSharedError)
	}
	if cssBuildError := builderForTest.BuildCSS(
		builder.CSSBuildOptions{
			BuildCriticalCSS: true,
			BuildNormalCSS:   true,
		},
	); cssBuildError != nil {
		t.Fatalf("BuildCSS returned error: %v", cssBuildError)
	}

	criticalClassifiedEvent := classifyEventWithWatcherAndBuilderForEventPipelineTests(
		fsnotify.Event{Name: criticalCSSFilePath, Op: fsnotify.Write},
		watcherForTest,
		builderForTest,
	)
	if criticalClassifiedEvent.FileType != eventpipeline.FileTypeCriticalCSS {
		t.Fatalf(
			"expected critical css file type, got %v",
			criticalClassifiedEvent.FileType,
		)
	}

	normalClassifiedEvent := classifyEventWithWatcherAndBuilderForEventPipelineTests(
		fsnotify.Event{Name: normalCSSFilePath, Op: fsnotify.Write},
		watcherForTest,
		builderForTest,
	)
	if normalClassifiedEvent.FileType != eventpipeline.FileTypeNormalCSS {
		t.Fatalf(
			"expected normal css file type, got %v",
			normalClassifiedEvent.FileType,
		)
	}

	sharedClassifiedEvent := classifyEventWithWatcherAndBuilderForEventPipelineTests(
		fsnotify.Event{Name: sharedCSSFilePath, Op: fsnotify.Write},
		watcherForTest,
		builderForTest,
	)
	if sharedClassifiedEvent.FileType != eventpipeline.FileTypeCriticalAndNormalCSS {
		t.Fatalf(
			"expected shared css file type %v, got %v",
			eventpipeline.FileTypeCriticalAndNormalCSS,
			sharedClassifiedEvent.FileType,
		)
	}
}

func TestClassifyEventWithWatcherAndBuilder_RespectsIgnoredFiles(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForEventClassificationTestsAtRoot(root)
	cfg.Watch.Exclude.Files = []string{"ignored.tmp"}

	watcherForTest, watcherCreateError := watch.NewWatcher(
		cfg,
		newDiscardLoggerForEventClassificationTests(),
	)
	if watcherCreateError != nil {
		t.Fatalf("NewWatcher returned error: %v", watcherCreateError)
	}
	defer watcherForTest.Close()

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForEventClassificationTests(),
	)
	defer builderForTest.Close()

	classifiedEvent := classifyEventWithWatcherAndBuilderForEventPipelineTests(
		fsnotify.Event{
			Name: filepath.Join(root, "ignored.tmp"),
			Op:   fsnotify.Write,
		},
		watcherForTest,
		builderForTest,
	)

	if !classifiedEvent.Ignored {
		t.Fatal("expected ignored file to classify with ignored=true")
	}
}

func TestClassifyEventWithWatcherAndBuilder_SiteStyleFileMatrix(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForEventClassificationTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Core.StaticAssetDirs.Public = filepath.Join(root, "frontend", "assets")
	cfg.Core.StaticAssetDirs.Private = filepath.Join(root, "backend", "assets")
	wavetest.SetCSSEntryFiles(
		cfg,
		filepath.Join(root, "frontend", "src", "styles", "main.critical.css"),
		filepath.Join(root, "frontend", "src", "styles", "main.css"),
	)
	cfg.Watch.Include = []wave.WatchedFile{
		{
			Pattern:                            "backend/assets/markdown/**/*.md",
			OnlyRunClientDefinedRevalidateFunc: true,
			SkipRebuildingNotification:         true,
		},
		{
			Pattern:                    "frontend/src/**/*vorma.routes.ts",
			RunOnChangeOnly:            true,
			SkipRebuildingNotification: true,
			OnChangeHooks: []wave.OnChangeHook{
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						return &wave.RefreshAction{
							ReloadBrowser: true,
							WaitForApp:    true,
							WaitForVite:   true,
						}, nil
					},
				},
			},
		},
		{
			Pattern:                    "frontend/assets/**/*",
			SkipRebuildingNotification: true,
			OnChangeHooks: []wave.OnChangeHook{
				{
					Cmd:    "go run ./backend/cmd/sync_docs --rewrite-only",
					Timing: wave.OnChangeStrategyPost,
				},
			},
		},
	}

	stylesDirectoryPath := filepath.Join(root, "frontend", "src", "styles")
	if err := os.MkdirAll(stylesDirectoryPath, 0o755); err != nil {
		t.Fatalf("create styles directory: %v", err)
	}
	if err := os.MkdirAll(cfg.Core.StaticAssetDirs.Public, 0o755); err != nil {
		t.Fatalf("create public static directory: %v", err)
	}
	if err := os.MkdirAll(
		filepath.Join(cfg.Core.StaticAssetDirs.Private, "markdown", "blog"),
		0o755,
	); err != nil {
		t.Fatalf("create private static markdown directory: %v", err)
	}
	if err := os.MkdirAll(
		filepath.Join(root, "frontend", "src", "routes"),
		0o755,
	); err != nil {
		t.Fatalf("create route source directory: %v", err)
	}
	if err := os.MkdirAll(
		filepath.Join(root, "frontend", "src", "lib"),
		0o755,
	); err != nil {
		t.Fatalf("create frontend lib directory: %v", err)
	}

	normalImportPath := filepath.Join(stylesDirectoryPath, "fonts.css")
	criticalImportPath := filepath.Join(
		stylesDirectoryPath,
		"critical_import.css",
	)
	tailwindPath := filepath.Join(stylesDirectoryPath, "tailwind.css")
	if err := os.WriteFile(
		cfg.Core.CSSEntryFiles.NonCritical,
		[]byte(`@import "./fonts.css"; body { color: blue; }`),
		0o644,
	); err != nil {
		t.Fatalf("write normal css entry: %v", err)
	}
	if err := os.WriteFile(
		cfg.Core.CSSEntryFiles.Critical,
		[]byte(`@import "./critical_import.css"; body { color: red; }`),
		0o644,
	); err != nil {
		t.Fatalf("write critical css entry: %v", err)
	}
	if err := os.WriteFile(
		normalImportPath,
		[]byte(`@font-face { src: url("/fonts/test.woff2") format("woff2"); }`),
		0o644,
	); err != nil {
		t.Fatalf("write normal css import: %v", err)
	}
	if err := os.WriteFile(
		criticalImportPath,
		[]byte(`.critical-import { display: block; }`),
		0o644,
	); err != nil {
		t.Fatalf("write critical css import: %v", err)
	}
	if err := os.WriteFile(
		tailwindPath,
		[]byte(`@import "tailwindcss";`),
		0o644,
	); err != nil {
		t.Fatalf("write tailwind css file: %v", err)
	}

	routeRegistryPath := filepath.Join(
		root,
		"frontend",
		"src",
		"routes",
		"core.vorma.routes.ts",
	)
	if err := os.WriteFile(
		routeRegistryPath,
		[]byte(`export const routeRegistry = [];`),
		0o644,
	); err != nil {
		t.Fatalf("write route registry file: %v", err)
	}

	vormaEntryPath := filepath.Join(root, "frontend", "src", "vorma.entry.tsx")
	if err := os.WriteFile(
		vormaEntryPath,
		[]byte(`export default function Entry() { return null; }`),
		0o644,
	); err != nil {
		t.Fatalf("write vorma entry file: %v", err)
	}

	randomFrontendTypeScriptPath := filepath.Join(
		root,
		"frontend",
		"src",
		"lib",
		"helpers.ts",
	)
	if err := os.WriteFile(
		randomFrontendTypeScriptPath,
		[]byte(`export const helper = () => 1;`),
		0o644,
	); err != nil {
		t.Fatalf("write random frontend ts file: %v", err)
	}

	publicStaticAssetPath := filepath.Join(
		cfg.Core.StaticAssetDirs.Public,
		"icon.svg",
	)
	if err := os.WriteFile(
		publicStaticAssetPath,
		[]byte(`<svg viewBox="0 0 1 1"></svg>`),
		0o644,
	); err != nil {
		t.Fatalf("write public static asset file: %v", err)
	}

	markdownBlogPath := filepath.Join(
		cfg.Core.StaticAssetDirs.Private,
		"markdown",
		"blog",
		"post.md",
	)
	if err := os.WriteFile(
		markdownBlogPath,
		[]byte(`# post`),
		0o644,
	); err != nil {
		t.Fatalf("write markdown blog file: %v", err)
	}

	watcherForTest, watcherCreateError := watch.NewWatcher(
		cfg,
		newDiscardLoggerForEventClassificationTests(),
	)
	if watcherCreateError != nil {
		t.Fatalf("watch.NewWatcher returned error: %v", watcherCreateError)
	}
	defer watcherForTest.Close()

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForEventClassificationTests(),
	)
	defer builderForTest.Close()

	if cssBuildError := builderForTest.BuildCSS(
		builder.CSSBuildOptions{
			BuildCriticalCSS: true,
			BuildNormalCSS:   true,
		},
	); cssBuildError != nil {
		t.Fatalf("BuildCSS returned error: %v", cssBuildError)
	}

	testCases := []struct {
		name            string
		path            string
		expectedType    eventpipeline.FileType
		expectedIgnored bool
		expectWatched   bool
		assertWatched   func(t *testing.T, watchedFile *wave.WatchedFile)
	}{
		{
			name:            "normal css entry",
			path:            cfg.Core.CSSEntryFiles.NonCritical,
			expectedType:    eventpipeline.FileTypeNormalCSS,
			expectedIgnored: false,
		},
		{
			name:            "upstream normal css import",
			path:            normalImportPath,
			expectedType:    eventpipeline.FileTypeNormalCSS,
			expectedIgnored: false,
		},
		{
			name:            "critical css entry",
			path:            cfg.Core.CSSEntryFiles.Critical,
			expectedType:    eventpipeline.FileTypeCriticalCSS,
			expectedIgnored: false,
		},
		{
			name:            "upstream critical css import",
			path:            criticalImportPath,
			expectedType:    eventpipeline.FileTypeCriticalCSS,
			expectedIgnored: false,
		},
		{
			name:            "tailwind css file not wave controlled",
			path:            tailwindPath,
			expectedType:    eventpipeline.FileTypeOther,
			expectedIgnored: true,
		},
		{
			name:            "random frontend ts file",
			path:            randomFrontendTypeScriptPath,
			expectedType:    eventpipeline.FileTypeOther,
			expectedIgnored: true,
		},
		{
			name:            "frontend vorma.entry.tsx file",
			path:            vormaEntryPath,
			expectedType:    eventpipeline.FileTypeOther,
			expectedIgnored: true,
		},
		{
			name:            "frontend route registry file",
			path:            routeRegistryPath,
			expectedType:    eventpipeline.FileTypeOther,
			expectedIgnored: false,
			expectWatched:   true,
			assertWatched: func(t *testing.T, watchedFile *wave.WatchedFile) {
				t.Helper()
				if !watchedFile.RunOnChangeOnly {
					t.Fatal(
						"expected route registry watched file to be run-on-change-only",
					)
				}
				if !watchedFile.SkipRebuildingNotification {
					t.Fatal(
						"expected route registry watched file to skip rebuilding notification",
					)
				}
				if len(watchedFile.OnChangeHooks) != 1 ||
					watchedFile.OnChangeHooks[0].Callback == nil {
					t.Fatalf(
						"expected route registry callback hook, got %#v",
						watchedFile.OnChangeHooks,
					)
				}
			},
		},
		{
			name:            "frontend static asset",
			path:            publicStaticAssetPath,
			expectedType:    eventpipeline.FileTypePublicStatic,
			expectedIgnored: false,
			expectWatched:   true,
			assertWatched: func(t *testing.T, watchedFile *wave.WatchedFile) {
				t.Helper()
				if !watchedFile.SkipRebuildingNotification {
					t.Fatal(
						"expected public static watched file to skip rebuilding notification",
					)
				}
				if len(watchedFile.OnChangeHooks) != 1 {
					t.Fatalf(
						"expected one public-static hook, got %#v",
						watchedFile.OnChangeHooks,
					)
				}
				if watchedFile.OnChangeHooks[0].Timing != wave.OnChangeStrategyPost {
					t.Fatalf(
						"expected public-static hook timing post, got %q",
						watchedFile.OnChangeHooks[0].Timing,
					)
				}
				if watchedFile.OnChangeHooks[0].Cmd == "" {
					t.Fatal("expected public-static hook command to be set")
				}
			},
		},
		{
			name:            "markdown blog file",
			path:            markdownBlogPath,
			expectedType:    eventpipeline.FileTypePrivateStatic,
			expectedIgnored: false,
			expectWatched:   true,
			assertWatched: func(t *testing.T, watchedFile *wave.WatchedFile) {
				t.Helper()
				if !watchedFile.OnlyRunClientDefinedRevalidateFunc {
					t.Fatal(
						"expected markdown watched file to request client revalidate",
					)
				}
				if !watchedFile.SkipRebuildingNotification {
					t.Fatal(
						"expected markdown watched file to skip rebuilding notification",
					)
				}
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			classifiedEvent := classifyEventWithWatcherAndBuilderForEventPipelineTests(
				fsnotify.Event{Name: testCase.path, Op: fsnotify.Write},
				watcherForTest,
				builderForTest,
			)

			if classifiedEvent.FileType != testCase.expectedType {
				t.Fatalf(
					"classified file type = %v, want %v",
					classifiedEvent.FileType,
					testCase.expectedType,
				)
			}
			if classifiedEvent.Ignored != testCase.expectedIgnored {
				t.Fatalf(
					"classified ignored = %v, want %v",
					classifiedEvent.Ignored,
					testCase.expectedIgnored,
				)
			}
			if testCase.expectWatched && classifiedEvent.WatchedFile == nil {
				t.Fatal("expected matched watched file, got nil")
			}
			if !testCase.expectWatched && classifiedEvent.WatchedFile != nil {
				t.Fatalf(
					"expected no watched file, got %#v",
					classifiedEvent.WatchedFile,
				)
			}
			if testCase.assertWatched != nil {
				testCase.assertWatched(t, classifiedEvent.WatchedFile)
			}
		})
	}
}

func TestClassifyWatcherEventsForProcessing_SiteStyleUnmanagedFrontendFilesDoNoWork(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForEventClassificationTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	wavetest.SetCSSEntryFiles(
		cfg,
		filepath.Join(root, "frontend", "src", "styles", "main.critical.css"),
		filepath.Join(root, "frontend", "src", "styles", "main.css"),
	)

	stylesDirectoryPath := filepath.Join(root, "frontend", "src", "styles")
	if err := os.MkdirAll(stylesDirectoryPath, 0o755); err != nil {
		t.Fatalf("create styles directory: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "frontend", "src", "lib"), 0o755); err != nil {
		t.Fatalf("create frontend lib directory: %v", err)
	}

	if err := os.WriteFile(
		cfg.Core.CSSEntryFiles.NonCritical,
		[]byte(`body { color: blue; }`),
		0o644,
	); err != nil {
		t.Fatalf("write normal css entry: %v", err)
	}
	if err := os.WriteFile(
		cfg.Core.CSSEntryFiles.Critical,
		[]byte(`body { color: red; }`),
		0o644,
	); err != nil {
		t.Fatalf("write critical css entry: %v", err)
	}

	tailwindPath := filepath.Join(stylesDirectoryPath, "tailwind.css")
	if err := os.WriteFile(
		tailwindPath,
		[]byte(`@import "tailwindcss";`),
		0o644,
	); err != nil {
		t.Fatalf("write tailwind css file: %v", err)
	}
	vormaEntryPath := filepath.Join(root, "frontend", "src", "vorma.entry.tsx")
	if err := os.WriteFile(
		vormaEntryPath,
		[]byte(`export default function Entry() { return null; }`),
		0o644,
	); err != nil {
		t.Fatalf("write vorma entry file: %v", err)
	}
	randomFrontendTypeScriptPath := filepath.Join(
		root,
		"frontend",
		"src",
		"lib",
		"helpers.ts",
	)
	if err := os.WriteFile(
		randomFrontendTypeScriptPath,
		[]byte(`export const helper = () => 1;`),
		0o644,
	); err != nil {
		t.Fatalf("write random frontend ts file: %v", err)
	}

	watcherForTest, watcherCreateError := watch.NewWatcher(
		cfg,
		newDiscardLoggerForEventClassificationTests(),
	)
	if watcherCreateError != nil {
		t.Fatalf("watch.NewWatcher returned error: %v", watcherCreateError)
	}
	defer watcherForTest.Close()

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForEventClassificationTests(),
	)
	defer builderForTest.Close()

	if cssBuildError := builderForTest.BuildCSS(
		builder.CSSBuildOptions{
			BuildCriticalCSS: true,
			BuildNormalCSS:   true,
		},
	); cssBuildError != nil {
		t.Fatalf("BuildCSS returned error: %v", cssBuildError)
	}

	classifiedEvents, configChanged := classifyWatcherEventsForProcessingForEventPipelineTests(
		cfg,
		[]fsnotify.Event{
			{Name: tailwindPath, Op: fsnotify.Write},
			{Name: vormaEntryPath, Op: fsnotify.Write},
			{Name: randomFrontendTypeScriptPath, Op: fsnotify.Write},
		},
		watcherForTest,
		builderForTest,
	)
	if configChanged {
		t.Fatal("expected configChanged=false for unmanaged frontend edits")
	}
	if len(classifiedEvents) != 0 {
		t.Fatalf(
			"expected unmanaged frontend edits to be filtered from processing, got %#v",
			classifiedEvents,
		)
	}
}

func newDiscardLoggerForEventClassificationTests() *slog.Logger {
	return wavetest.NewDiscardLogger()
}

func newParsedConfigForEventClassificationTestsAtRoot(
	root string,
) *wave.ParsedConfig {
	return wavetest.NewParsedConfigAtRoot(root)
}
