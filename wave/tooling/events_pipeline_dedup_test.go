package tooling

import (
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/internal/pathnorm"
)

func TestDeduplicateWatcherEventsByPath(
	t *testing.T,
) {
	deduplicatedEvents := deduplicateWatcherEventsByPath([]fsnotify.Event{
		{Name: "b.go", Op: fsnotify.Create},
		{Name: "a.go", Op: fsnotify.Write},
		{Name: "b.go", Op: fsnotify.Write},
	})

	if len(deduplicatedEvents) != 2 {
		t.Fatalf("deduplicated event count = %d, want 2", len(deduplicatedEvents))
	}
	if deduplicatedEvents[0].Name != "a.go" {
		t.Fatalf("deduplicated event[0] name = %q, want a.go", deduplicatedEvents[0].Name)
	}
	if deduplicatedEvents[1].Name != "b.go" {
		t.Fatalf("deduplicated event[1] name = %q, want b.go", deduplicatedEvents[1].Name)
	}
	if !deduplicatedEvents[1].Has(fsnotify.Write) {
		t.Fatalf("deduplicated event[1] op = %v, want write", deduplicatedEvents[1].Op)
	}
	if !deduplicatedEvents[1].Has(fsnotify.Create) {
		t.Fatalf("deduplicated event[1] op = %v, want create", deduplicatedEvents[1].Op)
	}
}

func TestDeduplicateWatcherEventsByPathMergesAllOps(
	t *testing.T,
) {
	deduplicatedEvents := deduplicateWatcherEventsByPath([]fsnotify.Event{
		{Name: "a.go", Op: fsnotify.Create},
		{Name: "a.go", Op: fsnotify.Write},
		{Name: "a.go", Op: fsnotify.Remove},
	})

	if len(deduplicatedEvents) != 1 {
		t.Fatalf("deduplicated event count = %d, want 1", len(deduplicatedEvents))
	}
	if !deduplicatedEvents[0].Has(fsnotify.Create) {
		t.Fatalf("deduplicated op = %v, want create", deduplicatedEvents[0].Op)
	}
	if !deduplicatedEvents[0].Has(fsnotify.Write) {
		t.Fatalf("deduplicated op = %v, want write", deduplicatedEvents[0].Op)
	}
	if !deduplicatedEvents[0].Has(fsnotify.Remove) {
		t.Fatalf("deduplicated op = %v, want remove", deduplicatedEvents[0].Op)
	}
}

func TestDeduplicateWatcherEventsByPath_NormalizesPathShapeVariants(
	t *testing.T,
) {
	root := t.TempDir()
	canonicalPath := filepath.Join(root, "backend", "main.go")
	pathWithDotSegment := filepath.Join(root, "backend", ".", "main.go")

	deduplicatedEvents := deduplicateWatcherEventsByPath([]fsnotify.Event{
		{Name: canonicalPath, Op: fsnotify.Create},
		{Name: pathWithDotSegment, Op: fsnotify.Write},
	})

	if len(deduplicatedEvents) != 1 {
		t.Fatalf("deduplicated event count = %d, want 1", len(deduplicatedEvents))
	}
	if deduplicatedEvents[0].Name != canonicalPath {
		t.Fatalf("deduplicated event name = %q, want %q", deduplicatedEvents[0].Name, canonicalPath)
	}
	if !deduplicatedEvents[0].Has(fsnotify.Create) || !deduplicatedEvents[0].Has(fsnotify.Write) {
		t.Fatalf("deduplicated op = %v, want create+write", deduplicatedEvents[0].Op)
	}
}

func TestDeduplicateWatcherEventsByPath_MergesSymlinkAliasPaths(
	t *testing.T,
) {
	root := t.TempDir()
	targetDirectoryPath := filepath.Join(root, "target")
	aliasDirectoryPath := filepath.Join(root, "alias")
	targetFilePath := filepath.Join(targetDirectoryPath, "styles.css")
	aliasFilePath := filepath.Join(aliasDirectoryPath, "styles.css")

	if err := os.MkdirAll(targetDirectoryPath, 0o755); err != nil {
		t.Fatalf("failed creating target directory: %v", err)
	}
	if err := os.WriteFile(targetFilePath, []byte("body{}"), 0o644); err != nil {
		t.Fatalf("failed writing target file: %v", err)
	}
	if err := os.Symlink(targetDirectoryPath, aliasDirectoryPath); err != nil {
		t.Fatalf("failed creating directory symlink: %v", err)
	}

	deduplicatedEvents := deduplicateWatcherEventsByPath([]fsnotify.Event{
		{Name: aliasFilePath, Op: fsnotify.Create},
		{Name: targetFilePath, Op: fsnotify.Write},
	})

	if len(deduplicatedEvents) != 1 {
		t.Fatalf("deduplicated event count = %d, want 1", len(deduplicatedEvents))
	}
	if !pathnorm.PathsReferToSameLocation(deduplicatedEvents[0].Name, targetFilePath) {
		t.Fatalf(
			"expected deduplicated path %q to refer to same location as %q",
			deduplicatedEvents[0].Name,
			targetFilePath,
		)
	}
	if !deduplicatedEvents[0].Has(fsnotify.Create) || !deduplicatedEvents[0].Has(fsnotify.Write) {
		t.Fatalf("deduplicated op = %v, want create+write", deduplicatedEvents[0].Op)
	}
}

func TestDeduplicateWatcherEventsByPath_MergesMissingFileAliasPaths(
	t *testing.T,
) {
	root := t.TempDir()
	targetDirectoryPath := filepath.Join(root, "target")
	aliasDirectoryPath := filepath.Join(root, "alias")
	targetMissingFilePath := filepath.Join(targetDirectoryPath, "missing.css")
	aliasMissingFilePath := filepath.Join(aliasDirectoryPath, "missing.css")

	if err := os.MkdirAll(targetDirectoryPath, 0o755); err != nil {
		t.Fatalf("failed creating target directory: %v", err)
	}
	if err := os.Symlink(targetDirectoryPath, aliasDirectoryPath); err != nil {
		t.Fatalf("failed creating directory symlink: %v", err)
	}

	deduplicatedEvents := deduplicateWatcherEventsByPath([]fsnotify.Event{
		{Name: aliasMissingFilePath, Op: fsnotify.Remove},
		{Name: targetMissingFilePath, Op: fsnotify.Remove},
	})

	if len(deduplicatedEvents) != 1 {
		t.Fatalf("deduplicated event count = %d, want 1", len(deduplicatedEvents))
	}
	if !deduplicatedEvents[0].Has(fsnotify.Remove) {
		t.Fatalf("deduplicated op = %v, want remove", deduplicatedEvents[0].Op)
	}
	deduplicatedAliasKey := missingFileAliasKeyForWatcherEventDeduplication(deduplicatedEvents[0].Name)
	targetAliasKey := missingFileAliasKeyForWatcherEventDeduplication(targetMissingFilePath)
	if deduplicatedAliasKey == "" || targetAliasKey == "" || deduplicatedAliasKey != targetAliasKey {
		t.Fatalf(
			"expected deduplicated path %q to resolve to same missing file alias key as %q",
			deduplicatedEvents[0].Name,
			targetMissingFilePath,
		)
	}
}

func TestDeduplicateWatcherEventsByPath_PrefersFirstSeenAliasPathKey(
	t *testing.T,
) {
	root := t.TempDir()
	targetDirectoryPath := filepath.Join(root, "target")
	aliasDirectoryPath := filepath.Join(root, "alias")
	targetFilePath := filepath.Join(targetDirectoryPath, "styles.css")
	aliasFilePath := filepath.Join(aliasDirectoryPath, "styles.css")

	if err := os.MkdirAll(targetDirectoryPath, 0o755); err != nil {
		t.Fatalf("failed creating target directory: %v", err)
	}
	if err := os.WriteFile(targetFilePath, []byte("body{}"), 0o644); err != nil {
		t.Fatalf("failed writing target file: %v", err)
	}
	if err := os.Symlink(targetDirectoryPath, aliasDirectoryPath); err != nil {
		t.Fatalf("failed creating directory symlink: %v", err)
	}

	testCases := []struct {
		name             string
		events           []fsnotify.Event
		expectedPathKey  string
		expectedEventOps fsnotify.Op
	}{
		{
			name: "alias key wins when alias appears first",
			events: []fsnotify.Event{
				{Name: aliasFilePath, Op: fsnotify.Create},
				{Name: targetFilePath, Op: fsnotify.Write},
			},
			expectedPathKey:  pathnorm.Absolute(aliasFilePath),
			expectedEventOps: fsnotify.Create | fsnotify.Write,
		},
		{
			name: "target key wins when target appears first",
			events: []fsnotify.Event{
				{Name: targetFilePath, Op: fsnotify.Create},
				{Name: aliasFilePath, Op: fsnotify.Write},
			},
			expectedPathKey:  pathnorm.Absolute(targetFilePath),
			expectedEventOps: fsnotify.Create | fsnotify.Write,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deduplicatedEvents := deduplicateWatcherEventsByPath(testCase.events)
			if len(deduplicatedEvents) != 1 {
				t.Fatalf("deduplicated event count = %d, want 1", len(deduplicatedEvents))
			}
			if deduplicatedEvents[0].Name != testCase.expectedPathKey {
				t.Fatalf(
					"deduplicated path key = %q, want %q",
					deduplicatedEvents[0].Name,
					testCase.expectedPathKey,
				)
			}
			if deduplicatedEvents[0].Op != testCase.expectedEventOps {
				t.Fatalf(
					"deduplicated event ops = %v, want %v",
					deduplicatedEvents[0].Op,
					testCase.expectedEventOps,
				)
			}
		})
	}
}

func TestWatcherEventDeduplicationIndex_MemoizesCanonicalAndAliasResolution(
	t *testing.T,
) {
	root := t.TempDir()
	targetDirectoryPath := filepath.Join(root, "target")
	aliasDirectoryPath := filepath.Join(root, "alias")
	targetFilePath := filepath.Join(targetDirectoryPath, "styles.css")
	aliasFilePath := filepath.Join(aliasDirectoryPath, "styles.css")
	aliasMissingFilePath := filepath.Join(aliasDirectoryPath, "missing.css")

	if err := os.MkdirAll(targetDirectoryPath, 0o755); err != nil {
		t.Fatalf("failed creating target directory: %v", err)
	}
	if err := os.WriteFile(targetFilePath, []byte("body{}"), 0o644); err != nil {
		t.Fatalf("failed writing target file: %v", err)
	}
	if err := os.Symlink(targetDirectoryPath, aliasDirectoryPath); err != nil {
		t.Fatalf("failed creating directory symlink: %v", err)
	}

	index := newWatcherEventDeduplicationIndex()
	absoluteAliasFilePath := pathnorm.Absolute(aliasFilePath)
	absoluteAliasMissingFilePath := pathnorm.Absolute(aliasMissingFilePath)

	canonicalPathFirst := index.resolveCanonicalPathForAbsolutePath(absoluteAliasFilePath)
	canonicalPathSecond := index.resolveCanonicalPathForAbsolutePath(absoluteAliasFilePath)
	if canonicalPathFirst == "" || canonicalPathSecond == "" || canonicalPathFirst != canonicalPathSecond {
		t.Fatalf(
			"expected stable canonical path memoization for %q, got first=%q second=%q",
			absoluteAliasFilePath,
			canonicalPathFirst,
			canonicalPathSecond,
		)
	}
	if len(index.canonicalPathByAbsolutePath) != 1 {
		t.Fatalf(
			"expected one canonical path cache entry, got %#v",
			index.canonicalPathByAbsolutePath,
		)
	}

	aliasKeyFirst := index.resolveMissingAliasKeyForAbsolutePath(absoluteAliasMissingFilePath)
	aliasKeySecond := index.resolveMissingAliasKeyForAbsolutePath(absoluteAliasMissingFilePath)
	if aliasKeyFirst == "" || aliasKeySecond == "" || aliasKeyFirst != aliasKeySecond {
		t.Fatalf(
			"expected stable missing-file alias key memoization for %q, got first=%q second=%q",
			absoluteAliasMissingFilePath,
			aliasKeyFirst,
			aliasKeySecond,
		)
	}
	if len(index.missingAliasKeyByAbsolutePath) != 1 {
		t.Fatalf(
			"expected one missing alias key cache entry, got %#v",
			index.missingAliasKeyByAbsolutePath,
		)
	}

	rootAliasKey := index.resolveMissingAliasKeyForAbsolutePath(string(filepath.Separator))
	if rootAliasKey != "" {
		t.Fatalf("expected empty alias key for root path, got %q", rootAliasKey)
	}
	if cachedRootAliasKey := index.missingAliasKeyByAbsolutePath[string(filepath.Separator)]; cachedRootAliasKey != missingAliasKeyResolutionEmptySentinel {
		t.Fatalf(
			"expected root path to cache empty alias sentinel %q, got %q",
			missingAliasKeyResolutionEmptySentinel,
			cachedRootAliasKey,
		)
	}
}

func TestDeduplicateWatcherEventsByPath_PropertyContracts(t *testing.T) {
	root := t.TempDir()
	targetDirectoryPath := filepath.Join(root, "target")
	aliasDirectoryPath := filepath.Join(root, "alias")
	relativeDirectoryPath := filepath.Join(root, "relative")
	targetFilePath := filepath.Join(targetDirectoryPath, "styles.css")
	aliasFilePath := filepath.Join(aliasDirectoryPath, "styles.css")
	targetMissingFilePath := filepath.Join(targetDirectoryPath, "missing.css")
	aliasMissingFilePath := filepath.Join(aliasDirectoryPath, "missing.css")
	relativeFilePath := filepath.Join(relativeDirectoryPath, "note.txt")
	relativeDotPath := filepath.Join(relativeDirectoryPath, ".", "note.txt")

	if err := os.MkdirAll(targetDirectoryPath, 0o755); err != nil {
		t.Fatalf("failed creating target directory: %v", err)
	}
	if err := os.MkdirAll(relativeDirectoryPath, 0o755); err != nil {
		t.Fatalf("failed creating relative directory: %v", err)
	}
	if err := os.WriteFile(targetFilePath, []byte("body{}"), 0o644); err != nil {
		t.Fatalf("failed writing target file: %v", err)
	}
	if err := os.WriteFile(relativeFilePath, []byte("note"), 0o644); err != nil {
		t.Fatalf("failed writing relative file: %v", err)
	}
	if err := os.Symlink(targetDirectoryPath, aliasDirectoryPath); err != nil {
		t.Fatalf("failed creating directory symlink: %v", err)
	}

	pathPool := []string{
		targetFilePath,
		aliasFilePath,
		targetMissingFilePath,
		aliasMissingFilePath,
		relativeFilePath,
		relativeDotPath,
		filepath.Join(root, "a", "..", "a", "main.go"),
		filepath.Join(root, "logs", "app.log"),
	}
	operationPool := []fsnotify.Op{
		fsnotify.Create,
		fsnotify.Write,
		fsnotify.Remove,
		fsnotify.Rename,
		fsnotify.Chmod,
	}
	seeds := []int64{1, 7, 42}

	for _, seed := range seeds {
		seedValue := seed
		t.Run("seed_"+strconv.FormatInt(seedValue, 10), func(t *testing.T) {
			randomGenerator := rand.New(rand.NewSource(seedValue))
			events := make([]fsnotify.Event, 0, 256)
			for i := 0; i < 256; i++ {
				events = append(events, fsnotify.Event{
					Name: pathPool[randomGenerator.Intn(len(pathPool))],
					Op:   operationPool[randomGenerator.Intn(len(operationPool))],
				})
			}

			deduplicatedEvents := deduplicateWatcherEventsByPath(events)

			if !sort.SliceIsSorted(deduplicatedEvents, func(i int, j int) bool {
				return deduplicatedEvents[i].Name < deduplicatedEvents[j].Name
			}) {
				t.Fatalf("expected deduplicated events to be sorted by path, got %#v", deduplicatedEvents)
			}

			idempotentDeduplicatedEvents := deduplicateWatcherEventsByPath(deduplicatedEvents)
			if !reflect.DeepEqual(idempotentDeduplicatedEvents, deduplicatedEvents) {
				t.Fatalf(
					"expected deduplication to be idempotent, first=%#v second=%#v",
					deduplicatedEvents,
					idempotentDeduplicatedEvents,
				)
			}

			for _, originalEvent := range events {
				matchedOutputEvent := false
				for _, deduplicatedEvent := range deduplicatedEvents {
					if !pathsEquivalentForDedupContract(originalEvent.Name, deduplicatedEvent.Name) {
						continue
					}
					if deduplicatedEvent.Op&originalEvent.Op != originalEvent.Op {
						continue
					}
					matchedOutputEvent = true
					break
				}
				if !matchedOutputEvent {
					t.Fatalf(
						"expected original event %#v to be represented in deduplicated output %#v",
						originalEvent,
						deduplicatedEvents,
					)
				}
			}
		})
	}
}

func pathsEquivalentForDedupContract(pathA string, pathB string) bool {
	normalizedPathA := normalizeWatcherEventPathForDeduplication(pathA)
	normalizedPathB := normalizeWatcherEventPathForDeduplication(pathB)
	if normalizedPathA == normalizedPathB {
		return true
	}

	if filepath.IsAbs(normalizedPathA) &&
		filepath.IsAbs(normalizedPathB) &&
		pathnorm.PathsReferToSameLocation(normalizedPathA, normalizedPathB) {
		return true
	}

	if filepath.IsAbs(normalizedPathA) && filepath.IsAbs(normalizedPathB) {
		aliasKeyA := missingFileAliasKeyForWatcherEventDeduplication(normalizedPathA)
		aliasKeyB := missingFileAliasKeyForWatcherEventDeduplication(normalizedPathB)
		if aliasKeyA != "" && aliasKeyA == aliasKeyB {
			return true
		}
	}

	return false
}

func TestClassifyWatcherEventsForProcessingDoesNotCollapseImplicitFileTypes(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)

	publicStaticFilePath := filepath.Join(cfg.Core.StaticAssetDirs.Public, "logo.svg")
	goFilePath := filepath.Join(root, "backend", "main.go")

	if err := os.MkdirAll(filepath.Dir(publicStaticFilePath), 0755); err != nil {
		t.Fatalf("failed creating public static file directory: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(goFilePath), 0755); err != nil {
		t.Fatalf("failed creating go file directory: %v", err)
	}
	if err := os.WriteFile(publicStaticFilePath, []byte("logo"), 0644); err != nil {
		t.Fatalf("failed writing public static file: %v", err)
	}
	if err := os.WriteFile(goFilePath, []byte("package main"), 0644); err != nil {
		t.Fatalf("failed writing go file: %v", err)
	}

	watcher, err := NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
	}
	defer watcher.Close()

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	serverForTest := &server{
		cfg: cfg,
		log: newDiscardLogger(),
	}

	classifiedEvents, configChanged := serverForTest.classifyWatcherEventsForProcessing(
		[]fsnotify.Event{
			{Name: publicStaticFilePath, Op: fsnotify.Write},
			{Name: goFilePath, Op: fsnotify.Write},
		},
		watcher,
		builder,
	)

	if configChanged {
		t.Fatal("expected configChanged=false for non-config files")
	}
	if len(classifiedEvents) != 2 {
		t.Fatalf("classified event count = %d, want 2", len(classifiedEvents))
	}

	foundGoEvent := false
	foundPublicStaticEvent := false
	for _, classifiedEventForProcessing := range classifiedEvents {
		if classifiedEventForProcessing.fileType == fileTypeGo {
			foundGoEvent = true
		}
		if classifiedEventForProcessing.fileType == fileTypePublicStatic {
			foundPublicStaticEvent = true
		}
	}

	if !foundGoEvent {
		t.Fatal("expected go event classification to be preserved")
	}
	if !foundPublicStaticEvent {
		t.Fatal("expected public static event classification to be preserved")
	}
}

func TestClassifyWatcherEventsForProcessingPreservesSharedPatternEvents(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Watch.Include = []wave.WatchedFile{
		{
			Pattern:         "**/*",
			RunOnChangeOnly: true,
		},
	}

	goFilePath := filepath.Join(root, "backend", "main.go")
	textFilePath := filepath.Join(root, "backend", "notes.txt")

	if err := os.MkdirAll(filepath.Dir(goFilePath), 0755); err != nil {
		t.Fatalf("failed creating go file directory: %v", err)
	}
	if err := os.WriteFile(goFilePath, []byte("package main"), 0644); err != nil {
		t.Fatalf("failed writing go file: %v", err)
	}
	if err := os.WriteFile(textFilePath, []byte("notes"), 0644); err != nil {
		t.Fatalf("failed writing text file: %v", err)
	}

	watcher, err := NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
	}
	defer watcher.Close()

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	serverForTest := &server{
		cfg: cfg,
		log: newDiscardLogger(),
	}

	classifiedEvents, configChanged := serverForTest.classifyWatcherEventsForProcessing(
		[]fsnotify.Event{
			{Name: goFilePath, Op: fsnotify.Write},
			{Name: textFilePath, Op: fsnotify.Write},
		},
		watcher,
		builder,
	)

	if configChanged {
		t.Fatal("expected configChanged=false for non-config files")
	}
	if len(classifiedEvents) != 2 {
		t.Fatalf("classified event count = %d, want 2", len(classifiedEvents))
	}
}

func TestBuildEventHooksForProcessingDeduplicatesHooksByPattern(
	t *testing.T,
) {
	classifiedEvents := []classifiedEvent{
		{
			event:    fsnotify.Event{Name: "a.go", Op: fsnotify.Write},
			fileType: fileTypeGo,
			watchedFile: &wave.WatchedFile{
				Pattern: "**/*",
			},
		},
		{
			event:    fsnotify.Event{Name: "b.txt", Op: fsnotify.Write},
			fileType: fileTypeOther,
			watchedFile: &wave.WatchedFile{
				Pattern: "**/*",
			},
		},
	}

	eventsWithHooks := buildEventHooksForProcessing(classifiedEvents)
	if len(eventsWithHooks) != 2 {
		t.Fatalf("eventsWithHooks count = %d, want 2", len(eventsWithHooks))
	}
	if eventsWithHooks[0].skipDuplicateHooks {
		t.Fatal("expected first event with shared pattern to run hooks")
	}
	if !eventsWithHooks[1].skipDuplicateHooks {
		t.Fatal("expected second event with shared pattern to skip duplicate hooks")
	}
	if !anyEventNeedsHardReload(eventsWithHooks) {
		t.Fatal("expected batchNeedsAppStop=true when one event requires hard reload")
	}
	if got, want := eventsWithHooks[0].hookCtx.ChangedFilePaths, []string{"a.go", "b.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("first hook context changed file paths = %v, want %v", got, want)
	}
	if got, want := eventsWithHooks[1].hookCtx.ChangedFilePaths, []string{"a.go", "b.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("second hook context changed file paths = %v, want %v", got, want)
	}
}

func TestBuildEventHooksForProcessingNoPatternUsesSingleChangedFilePath(
	t *testing.T,
) {
	classifiedEvents := []classifiedEvent{
		{
			event:    fsnotify.Event{Name: "/tmp/standalone.go", Op: fsnotify.Write},
			fileType: fileTypeGo,
		},
	}

	eventsWithHooks := buildEventHooksForProcessing(classifiedEvents)
	if len(eventsWithHooks) != 1 {
		t.Fatalf("eventsWithHooks count = %d, want 1", len(eventsWithHooks))
	}
	if got, want := eventsWithHooks[0].hookCtx.ChangedFilePaths, []string{"/tmp/standalone.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("hook context changed file paths = %v, want %v", got, want)
	}
}

func TestProcessEvents_DeduplicatesHooksByPatternForMixedFileTypes(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true

	var callbackCount int32
	cfg.Watch.Include = []wave.WatchedFile{
		{
			Pattern:         "**/*",
			RunOnChangeOnly: true,
			OnChangeHooks: []wave.OnChangeHook{
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						atomic.AddInt32(&callbackCount, 1)
						return nil, nil
					},
				},
			},
		},
	}
	cfg.Dist = wave.DistLayout{Root: cfg.Core.DistDir}

	goFilePath := filepath.Join(root, "backend", "main.go")
	textFilePath := filepath.Join(root, "backend", "notes.txt")

	if err := os.MkdirAll(filepath.Dir(goFilePath), 0755); err != nil {
		t.Fatalf("failed creating go file directory: %v", err)
	}
	if err := os.WriteFile(goFilePath, []byte("package main"), 0644); err != nil {
		t.Fatalf("failed writing go file: %v", err)
	}
	if err := os.WriteFile(textFilePath, []byte("notes"), 0644); err != nil {
		t.Fatalf("failed writing text file: %v", err)
	}

	watcher, err := NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
	}
	defer watcher.Close()

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	serverForTest := &server{
		cfg:       cfg,
		log:       newDiscardLogger(),
		watcher:   watcher,
		builder:   builder,
		restartCh: make(chan restartRequest, 1),
	}

	serverForTest.processEvents([]fsnotify.Event{
		{Name: goFilePath, Op: fsnotify.Write},
		{Name: textFilePath, Op: fsnotify.Write},
	})

	if got := atomic.LoadInt32(&callbackCount); got != 1 {
		t.Fatalf("expected callback to run once for shared pattern across mixed file types, got %d", got)
	}
}
