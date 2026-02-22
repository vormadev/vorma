package classification_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave/tooling/internal/watch/classification"
)

func TestDeriveWatcherEventPreClassificationDecision(t *testing.T) {
	dependencies := classification.PathClassifierDependencies{
		LockFileName: ".wave.lock",
		IsIgnoredPathFunc: func(path string) bool {
			return filepath.Base(path) == "ignored.txt"
		},
	}

	emptyPathDecision := classification.DerivePreClassificationDecision(
		fsnotify.Event{Name: "", Op: fsnotify.Write},
		dependencies,
	)
	if emptyPathDecision.IncludeEvent {
		t.Fatalf("expected empty-path event to be excluded, got %#v", emptyPathDecision)
	}
	if emptyPathDecision.IgnoreReason != "empty_path" {
		t.Fatalf(
			"expected ignore_reason=empty_path, got %#v",
			emptyPathDecision,
		)
	}

	lockFileDecision := classification.DerivePreClassificationDecision(
		fsnotify.Event{Name: filepath.Join("tmp", ".wave.lock"), Op: fsnotify.Write},
		dependencies,
	)
	if lockFileDecision.IncludeEvent || !lockFileDecision.LooksLikeLockIO {
		t.Fatalf(
			"expected lock-file event to be excluded with LooksLikeLockIO=true, got %#v",
			lockFileDecision,
		)
	}

	tempFileDecision := classification.DerivePreClassificationDecision(
		fsnotify.Event{Name: filepath.Join("tmp", "editor.tmp"), Op: fsnotify.Write},
		dependencies,
	)
	if tempFileDecision.IncludeEvent || !tempFileDecision.LooksLikeTempIO {
		t.Fatalf(
			"expected temp-file event to be excluded with LooksLikeTempIO=true, got %#v",
			tempFileDecision,
		)
	}

	ignoredPathDecision := classification.DerivePreClassificationDecision(
		fsnotify.Event{Name: filepath.Join("tmp", "ignored.txt"), Op: fsnotify.Write},
		dependencies,
	)
	if ignoredPathDecision.IncludeEvent {
		t.Fatalf(
			"expected ignored-path event to be excluded, got %#v",
			ignoredPathDecision,
		)
	}
	if ignoredPathDecision.IgnoreReason != "ignored_path" {
		t.Fatalf(
			"expected ignore_reason=ignored_path, got %#v",
			ignoredPathDecision,
		)
	}

	regularFileDecision := classification.DerivePreClassificationDecision(
		fsnotify.Event{Name: filepath.Join("tmp", "app.go"), Op: fsnotify.Write},
		dependencies,
	)
	if !regularFileDecision.IncludeEvent {
		t.Fatalf("expected regular file write to be included, got %#v", regularFileDecision)
	}
	if regularFileDecision.UnderlyingKind != classification.EventKindWrite {
		t.Fatalf(
			"expected underlying kind write, got %#v",
			regularFileDecision,
		)
	}
}

func TestDeriveWatcherEventPreClassificationDecisionForNonConfigEvent(
	t *testing.T,
) {
	dependencies := classification.PathClassifierDependencies{}

	unknownEventDecision := classification.DerivePreClassificationDecision(
		fsnotify.Event{Name: "app.go"},
		dependencies,
	)
	if !unknownEventDecision.IncludeEvent ||
		unknownEventDecision.UnderlyingKind != classification.EventKindUnknown ||
		!unknownEventDecision.FromUnknownEvent ||
		!unknownEventDecision.ShouldSkipStat {
		t.Fatalf(
			"expected unknown event to remain included with unknown metadata flags, got %#v",
			unknownEventDecision,
		)
	}

	createEventDecision := classification.DerivePreClassificationDecision(
		fsnotify.Event{Name: "app.go", Op: fsnotify.Create},
		dependencies,
	)
	if !createEventDecision.IncludeEvent ||
		createEventDecision.UnderlyingKind != classification.EventKindCreate ||
		createEventDecision.FromUnknownEvent ||
		createEventDecision.ShouldSkipStat {
		t.Fatalf(
			"expected create event to be included with create kind and no unknown flags, got %#v",
			createEventDecision,
		)
	}
}

func TestBuildWatcherEventPreClassificationPlanFromEvents(t *testing.T) {
	dependencies := classification.PathClassifierDependencies{
		LockFileName: ".wave.lock",
		IsIgnoredPathFunc: func(path string) bool {
			return filepath.Base(path) == "ignored.txt"
		},
	}

	events := []fsnotify.Event{
		{Name: filepath.Join("tmp", ".wave.lock"), Op: fsnotify.Write},
		{Name: filepath.Join("tmp", "ignored.txt"), Op: fsnotify.Write},
		{Name: filepath.Join("tmp", "editor.tmp"), Op: fsnotify.Write},
		{Name: filepath.Join("tmp", "app.go"), Op: fsnotify.Write},
	}

	includedEvents := buildWatcherEventPreClassificationPlanFromEventsForTest(
		events,
		dependencies,
	)
	expectedIncludedEvents := []fsnotify.Event{
		{Name: filepath.Join("tmp", "app.go"), Op: fsnotify.Write},
	}
	if !reflect.DeepEqual(includedEvents, expectedIncludedEvents) {
		t.Fatalf("included events=%#v, want %#v", includedEvents, expectedIncludedEvents)
	}
}

func TestDeriveWatcherEventPreClassificationStepResult(t *testing.T) {
	missingPathWriteEvent := fsnotify.Event{
		Name: filepath.Join("tmp", "missing.txt"),
		Op:   fsnotify.Write,
	}
	if classification.ShouldSuppressEventForMissingPath(
		missingPathWriteEvent,
		os.ErrNotExist,
	) {
		t.Fatalf(
			"expected missing-path write event to remain actionable, got suppression=true",
		)
	}

	missingPathRenameEvent := fsnotify.Event{
		Name: filepath.Join("tmp", "missing.txt"),
		Op:   fsnotify.Rename,
	}
	if !classification.ShouldSuppressEventForMissingPath(
		missingPathRenameEvent,
		os.ErrNotExist,
	) {
		t.Fatalf(
			"expected missing-path rename event to be suppressed, got suppression=false",
		)
	}
}

func TestWatcherEventClassificationProber_CachesConfigProbeByPath(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "backend", "wave.config.json")
	if makeDirectoryError := os.MkdirAll(filepath.Dir(configPath), 0o755); makeDirectoryError != nil {
		t.Fatalf("failed creating config directory: %v", makeDirectoryError)
	}
	if writeError := os.WriteFile(configPath, []byte("{}"), 0o644); writeError != nil {
		t.Fatalf("failed writing config file: %v", writeError)
	}

	equivalentConfigPath := filepath.Join(root, "backend", ".", "wave.config.json")
	if !classification.IsConfigurationPathChange(equivalentConfigPath, configPath) {
		t.Fatalf(
			"expected equivalent config path %q to match %q",
			equivalentConfigPath,
			configPath,
		)
	}
	if !classification.IsConfigurationPathChange(equivalentConfigPath, configPath) {
		t.Fatalf(
			"expected repeated config path check %q to remain true for %q",
			equivalentConfigPath,
			configPath,
		)
	}
}

func TestWatcherEventClassificationProber_CachesDirectoryProbeByPath(t *testing.T) {
	root := t.TempDir()
	directoryPath := filepath.Join(root, "assets")
	filePath := filepath.Join(root, "app.go")
	missingPath := filepath.Join(root, "missing")

	if makeDirectoryError := os.MkdirAll(directoryPath, 0o755); makeDirectoryError != nil {
		t.Fatalf("failed creating directory: %v", makeDirectoryError)
	}
	if writeError := os.WriteFile(filePath, []byte("package main"), 0o644); writeError != nil {
		t.Fatalf("failed writing regular file: %v", writeError)
	}

	if !classification.IsLikelyDirectoryChange(directoryPath) {
		t.Fatalf("expected IsLikelyDirectoryChange(%q)=true", directoryPath)
	}
	if classification.IsLikelyDirectoryChange(filePath) {
		t.Fatalf("expected IsLikelyDirectoryChange(%q)=false", filePath)
	}
	if classification.IsLikelyDirectoryChange(missingPath) {
		t.Fatalf("expected IsLikelyDirectoryChange(%q)=false", missingPath)
	}
}

func TestWatcherEventClassificationProber_SharesSingleSnapshotAcrossProbeTypes(
	t *testing.T,
) {
	root := t.TempDir()
	realPath := filepath.Join(root, "real.txt")
	if writeError := os.WriteFile(realPath, []byte("ok"), 0o644); writeError != nil {
		t.Fatalf("failed writing real file: %v", writeError)
	}
	symlinkPath := filepath.Join(root, "link.txt")
	if symlinkError := os.Symlink(realPath, symlinkPath); symlinkError != nil {
		t.Skipf("symlink not available in this environment: %v", symlinkError)
	}

	canonicalFromRealPath := classification.CanonicalizePathForEvent(realPath)
	canonicalFromSymlinkPath := classification.CanonicalizePathForEvent(symlinkPath)
	if canonicalFromRealPath == "" || canonicalFromSymlinkPath == "" {
		t.Fatalf(
			"expected canonical paths to be non-empty, got real=%q symlink=%q",
			canonicalFromRealPath,
			canonicalFromSymlinkPath,
		)
	}
	if canonicalFromRealPath != canonicalFromSymlinkPath {
		t.Fatalf(
			"expected canonical real/symlink paths to match, got real=%q symlink=%q",
			canonicalFromRealPath,
			canonicalFromSymlinkPath,
		)
	}
}

func TestDeriveWatcherEventPostClassificationDecision(t *testing.T) {
	ignoredDecision := classification.DerivePostClassificationDecision(true, false)
	if ignoredDecision.IncludeClassifiedEvent || !ignoredDecision.DroppedBecauseIgnored {
		t.Fatalf(
			"expected ignored event to be dropped because ignored, got %#v",
			ignoredDecision,
		)
	}

	chmodDecision := classification.DerivePostClassificationDecision(false, true)
	if chmodDecision.IncludeClassifiedEvent || !chmodDecision.DroppedBecauseChmod {
		t.Fatalf(
			"expected chmod-only event to be dropped because chmod, got %#v",
			chmodDecision,
		)
	}

	includedDecision := classification.DerivePostClassificationDecision(false, false)
	if !includedDecision.IncludeClassifiedEvent ||
		includedDecision.DroppedBecauseIgnored ||
		includedDecision.DroppedBecauseChmod {
		t.Fatalf(
			"expected non-ignored non-chmod event to be included, got %#v",
			includedDecision,
		)
	}
}

func TestShouldLogWatcherAddDirectoryError(t *testing.T) {
	if classification.ShouldLogWatcherAddDirectoryError(nil) {
		t.Fatal("expected nil add-directory error to be suppressed")
	}
	if classification.ShouldLogWatcherAddDirectoryError(os.ErrNotExist) {
		t.Fatal("expected os.ErrNotExist add-directory error to be suppressed")
	}
	if classification.ShouldLogWatcherAddDirectoryError(fs.ErrNotExist) {
		t.Fatal("expected fs.ErrNotExist add-directory error to be suppressed")
	}
	if classification.ShouldLogWatcherAddDirectoryError(syscall.ENOTDIR) {
		t.Fatal("expected ENOTDIR add-directory error to be suppressed")
	}
	if !classification.ShouldLogWatcherAddDirectoryError(errors.New("boom")) {
		t.Fatal("expected unknown add-directory error to be logged")
	}
}

func buildWatcherEventPreClassificationPlanFromEventsForTest(
	events []fsnotify.Event,
	dependencies classification.PathClassifierDependencies,
) []fsnotify.Event {
	includedEvents := make([]fsnotify.Event, 0, len(events))
	for _, watcherEvent := range events {
		preClassificationDecision := classification.DerivePreClassificationDecision(
			watcherEvent,
			dependencies,
		)
		if preClassificationDecision.IncludeEvent {
			includedEvents = append(includedEvents, watcherEvent)
		}
	}
	return includedEvents
}
