package dedup_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave/tooling/internal/watch/dedup"
)

func TestDeduplicateWatcherEvents_PrefersRemoveOverWrite(t *testing.T) {
	rawEvents := []fsnotify.Event{
		{Name: "/tmp/a.txt", Op: fsnotify.Write},
		{Name: "/tmp/a.txt", Op: fsnotify.Remove},
		{Name: "/tmp/b.txt", Op: fsnotify.Create},
	}

	result := dedup.DeduplicateWatcherEvents(rawEvents, dedup.DeduplicationPolicy{})
	if len(result.Events) != 2 {
		t.Fatalf("expected 2 deduplicated events, got %d", len(result.Events))
	}
	if len(result.DroppedPaths) == 0 {
		t.Fatal("expected dropped paths to include merged path")
	}

	var foundRemove bool
	for _, event := range result.Events {
		if event.Name == "/tmp/a.txt" && event.Has(fsnotify.Remove) {
			foundRemove = true
		}
	}
	if !foundRemove {
		t.Fatalf("expected /tmp/a.txt event to retain remove op, got %#v", result.Events)
	}
}

func TestDeduplicateWatcherEventsByPath_MergesOpsAndSorts(t *testing.T) {
	rawEvents := []fsnotify.Event{
		{Name: "/tmp/b.txt", Op: fsnotify.Write},
		{Name: "/tmp/a.txt", Op: fsnotify.Create},
		{Name: "/tmp/a.txt", Op: fsnotify.Write},
	}

	deduplicatedEvents := dedup.DeduplicateWatcherEventsByPath(rawEvents)
	if len(deduplicatedEvents) != 2 {
		t.Fatalf("expected 2 deduplicated events, got %d", len(deduplicatedEvents))
	}
	if deduplicatedEvents[0].Name != "/tmp/a.txt" || deduplicatedEvents[1].Name != "/tmp/b.txt" {
		t.Fatalf("expected path-sorted output, got %#v", deduplicatedEvents)
	}
	if !deduplicatedEvents[0].Has(fsnotify.Create) || !deduplicatedEvents[0].Has(fsnotify.Write) {
		t.Fatalf("expected merged ops for /tmp/a.txt, got %#v", deduplicatedEvents[0])
	}
}

func TestFilterAndGroupWatcherEvents(t *testing.T) {
	rawEvents := []fsnotify.Event{
		{Name: "/tmp/project/a.txt", Op: fsnotify.Write},
		{Name: "/tmp/project/nested/b.txt", Op: fsnotify.Create},
		{Name: "/tmp/other/c.txt", Op: fsnotify.Write},
	}

	filteredEvents := dedup.FilterEventsByPathPrefix(rawEvents, "/tmp/project")
	if len(filteredEvents) != 2 {
		t.Fatalf("expected 2 filtered events, got %d", len(filteredEvents))
	}

	groupedEvents := dedup.GroupEventsByDirectory(filteredEvents)
	if len(groupedEvents) != 2 {
		t.Fatalf("expected 2 grouped directories, got %d", len(groupedEvents))
	}
	if _, exists := groupedEvents["/tmp/project"]; !exists {
		t.Fatalf("expected grouped events to include /tmp/project, got %#v", groupedEvents)
	}
	if _, exists := groupedEvents["/tmp/project/nested"]; !exists {
		t.Fatalf("expected grouped events to include /tmp/project/nested, got %#v", groupedEvents)
	}
}

func TestBuildCanonicalPathAliasSetAndContains(t *testing.T) {
	absolutePath, absolutePathError := filepath.Abs("tmp/path.txt")
	if absolutePathError != nil {
		t.Fatalf("filepath.Abs returned error: %v", absolutePathError)
	}

	aliasSet := dedup.BuildCanonicalPathAliasSet(absolutePath)
	if len(aliasSet) == 0 {
		t.Fatal("expected non-empty alias set")
	}
	if !dedup.PathAliasContains(aliasSet, absolutePath) {
		t.Fatalf("expected alias set to contain absolute path %q", absolutePath)
	}
	if !dedup.PathAliasContains(aliasSet, strings.ToLower(absolutePath)) {
		t.Fatalf("expected alias set to contain lowercase path %q", strings.ToLower(absolutePath))
	}
}

func TestWatcherEventDeduplicationIndex_RecordAndResolve(t *testing.T) {
	root := t.TempDir()
	pathKey := filepath.Join(root, "gone.txt")
	index := dedup.NewWatcherEventDeduplicationIndex()
	index.RecordPathKey(pathKey)

	mergedEventOpsByPath := map[string]fsnotify.Op{
		pathKey: fsnotify.Remove,
	}
	queryPath := filepath.Clean(root) + string(filepath.Separator) + "child" + string(filepath.Separator) + ".." + string(filepath.Separator) + "gone.txt"
	resolvedPathKey := index.ResolveExistingPathKey(mergedEventOpsByPath, queryPath)
	if resolvedPathKey != pathKey {
		t.Fatalf("resolved path key = %q, want %q", resolvedPathKey, pathKey)
	}
}
