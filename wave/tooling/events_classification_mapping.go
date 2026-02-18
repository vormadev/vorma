package tooling

import (
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave"
)

// classifyEventWithWatcherAndBuilder maps a raw watcher event into file type,
// watch metadata, ignore status, and chmod-only state used by later phases.
func (s *server) classifyEventWithWatcherAndBuilder(
	watcherEvent fsnotify.Event,
	watcher *watcher,
	builder *Builder,
) classifiedEvent {
	classifiedEventForProcessing := classifiedEvent{event: watcherEvent}

	if watcherEvent.Name == "" {
		classifiedEventForProcessing.ignored = true
		return classifiedEventForProcessing
	}

	classifiedEventForProcessing.ignored = watcher.IsIgnoredFile(watcherEvent.Name)
	classifiedEventForProcessing.fileType = deriveInitialFileTypeForWatcherEvent(
		watcherEvent.Name,
		watcher,
		builder,
	)
	classifiedEventForProcessing.watchedFile = watcher.FindWatchedFile(watcherEvent.Name)
	classifiedEventForProcessing.fileType = deriveFileTypeWithWatchedFileOverrides(
		classifiedEventForProcessing.fileType,
		classifiedEventForProcessing.watchedFile,
	)
	classifiedEventForProcessing.ignored = deriveWatcherEventIgnoredStatus(
		classifiedEventForProcessing.ignored,
		classifiedEventForProcessing.fileType,
		classifiedEventForProcessing.watchedFile,
	)
	classifiedEventForProcessing.chmodOnly = isNonEmptyChmodOnly(watcherEvent)

	return classifiedEventForProcessing
}

// deriveInitialFileTypeForWatcherEvent resolves baseline file type before
// watched-file overrides are applied.
func deriveInitialFileTypeForWatcherEvent(
	watcherEventPath string,
	watcher *watcher,
	builder *Builder,
) fileType {
	isCriticalCSSFile := builder.IsCriticalCSSFile(watcherEventPath)
	isNormalCSSFile := builder.IsNormalCSSFile(watcherEventPath)

	if isCriticalCSSFile && isNormalCSSFile {
		return fileTypeCriticalAndNormalCSS
	}
	if isCriticalCSSFile {
		return fileTypeCriticalCSS
	}
	if isNormalCSSFile {
		return fileTypeNormalCSS
	}
	if filepath.Ext(watcherEventPath) == ".go" {
		return fileTypeGo
	}
	if watcher.IsPublicStaticFile(watcherEventPath) {
		return fileTypePublicStatic
	}
	if watcher.IsPrivateStaticFile(watcherEventPath) {
		return fileTypePrivateStatic
	}
	return fileTypeOther
}

// deriveFileTypeWithWatchedFileOverrides applies per-watched-file overrides
// after extension/pattern-based initial classification.
func deriveFileTypeWithWatchedFileOverrides(
	initialFileType fileType,
	watchedFile *wave.WatchedFile,
) fileType {
	if initialFileType == fileTypeGo &&
		watchedFile != nil &&
		watchedFile.TreatAsNonGo {
		return fileTypeOther
	}
	return initialFileType
}

// deriveWatcherEventIgnoredStatus resolves whether a classified event should be
// ignored before post-classification filtering.
func deriveWatcherEventIgnoredStatus(
	initialIgnoredStatus bool,
	resolvedFileType fileType,
	watchedFile *wave.WatchedFile,
) bool {
	if initialIgnoredStatus {
		return true
	}
	return resolvedFileType == fileTypeOther && watchedFile == nil
}
