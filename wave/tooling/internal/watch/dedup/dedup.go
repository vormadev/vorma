package dedup

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/internal/wavecore"
)

// PathEvent stores one watcher event after path normalization.
type PathEvent struct {
	NormalizedPath string
	Event          fsnotify.Event
}

// DedupeResult captures normalized events and dropped paths.
type DedupeResult struct {
	Events       []fsnotify.Event
	DroppedPaths []string
}

// DeduplicationPolicy controls event merge behavior.
type DeduplicationPolicy struct {
	PreferRenameOverCreate bool
	PreferRemoveOverWrite  bool
	SortStableByPath       bool
}

// policyWithDefaults applies stable dedup defaults.
func policyWithDefaults(policy DeduplicationPolicy) DeduplicationPolicy {
	resolvedPolicy := policy
	if !resolvedPolicy.PreferRenameOverCreate {
		resolvedPolicy.PreferRenameOverCreate = true
	}
	if !resolvedPolicy.PreferRemoveOverWrite {
		resolvedPolicy.PreferRemoveOverWrite = true
	}
	if !resolvedPolicy.SortStableByPath {
		resolvedPolicy.SortStableByPath = true
	}
	return resolvedPolicy
}

// NormalizeEventPath canonicalizes a watcher path for map-key deduplication.
func NormalizeEventPath(rawPath string) string {
	if strings.TrimSpace(rawPath) == "" {
		return ""
	}
	cleanedPath := filepath.Clean(rawPath)
	return strings.ReplaceAll(cleanedPath, "\\", "/")
}

// DeduplicateWatcherEvents merges event bursts by normalized path.
func DeduplicateWatcherEvents(
	rawEvents []fsnotify.Event,
	policy DeduplicationPolicy,
) DedupeResult {
	if len(rawEvents) == 0 {
		return DedupeResult{}
	}

	resolvedPolicy := policyWithDefaults(policy)
	latestByPath := make(map[string]fsnotify.Event, len(rawEvents))
	insertionOrderByPath := make(map[string]int, len(rawEvents))
	droppedPaths := make([]string, 0)
	deduplicationIndex := NewWatcherEventDeduplicationIndex()

	for eventIndex := range rawEvents {
		rawEvent := rawEvents[eventIndex]
		normalizedPath := NormalizeWatcherEventPathForDeduplication(rawEvent.Name)
		if normalizedPath == "" {
			continue
		}
		if existingPathKey := deduplicationIndex.ResolveExistingPathKey(
			mapPathOps(latestByPath),
			normalizedPath,
		); existingPathKey != "" {
			normalizedPath = existingPathKey
		} else {
			deduplicationIndex.RecordPathKey(normalizedPath)
		}
		rawEvent.Name = normalizedPath

		previousEvent, alreadySeen := latestByPath[normalizedPath]
		if !alreadySeen {
			latestByPath[normalizedPath] = rawEvent
			insertionOrderByPath[normalizedPath] = eventIndex
			continue
		}

		mergedEvent, replaced := MergeEventsForSamePath(
			previousEvent,
			rawEvent,
			resolvedPolicy,
		)
		if replaced {
			droppedPaths = append(droppedPaths, normalizedPath)
		}
		latestByPath[normalizedPath] = mergedEvent
	}

	dedupedEvents := make([]fsnotify.Event, 0, len(latestByPath))
	for normalizedPath := range latestByPath {
		dedupedEvents = append(dedupedEvents, latestByPath[normalizedPath])
	}

	if resolvedPolicy.SortStableByPath {
		sort.SliceStable(
			dedupedEvents,
			func(leftIndex int, rightIndex int) bool {
				leftPath := dedupedEvents[leftIndex].Name
				rightPath := dedupedEvents[rightIndex].Name
				leftOrder := insertionOrderByPath[leftPath]
				rightOrder := insertionOrderByPath[rightPath]
				if leftOrder != rightOrder {
					return leftOrder < rightOrder
				}
				return leftPath < rightPath
			},
		)
	}

	return DedupeResult{
		Events:       dedupedEvents,
		DroppedPaths: droppedPaths,
	}
}

func mapPathOps(
	eventsByPath map[string]fsnotify.Event,
) map[string]fsnotify.Op {
	operationsByPath := make(map[string]fsnotify.Op, len(eventsByPath))
	for pathKey, event := range eventsByPath {
		operationsByPath[pathKey] = event.Op
	}
	return operationsByPath
}

// MergeEventsForSamePath merges two events that target the same normalized path.
func MergeEventsForSamePath(
	previousEvent fsnotify.Event,
	incomingEvent fsnotify.Event,
	policy DeduplicationPolicy,
) (fsnotify.Event, bool) {
	resolvedPolicy := policyWithDefaults(policy)

	if shouldPreferIncomingEvent(previousEvent, incomingEvent, resolvedPolicy) {
		return incomingEvent, true
	}
	if shouldPreferPreviousEvent(previousEvent, incomingEvent, resolvedPolicy) {
		return previousEvent, false
	}

	combinedOperation := previousEvent.Op | incomingEvent.Op
	mergedEvent := fsnotify.Event{
		Name: previousEvent.Name,
		Op:   combinedOperation,
	}
	return mergedEvent, true
}

// shouldPreferIncomingEvent applies high-confidence replacement rules.
func shouldPreferIncomingEvent(
	previousEvent fsnotify.Event,
	incomingEvent fsnotify.Event,
	policy DeduplicationPolicy,
) bool {
	if policy.PreferRemoveOverWrite {
		if incomingEvent.Has(fsnotify.Remove) {
			return true
		}
		if incomingEvent.Has(fsnotify.Rename) &&
			previousEvent.Has(fsnotify.Write) {
			return true
		}
	}

	if policy.PreferRenameOverCreate {
		if previousEvent.Has(fsnotify.Create) &&
			incomingEvent.Has(fsnotify.Rename) {
			return true
		}
	}

	if incomingEvent.Has(fsnotify.Create) &&
		previousEvent.Has(fsnotify.Create) {
		return true
	}

	if incomingEvent.Has(fsnotify.Write) && previousEvent.Has(fsnotify.Write) {
		return true
	}

	return false
}

// shouldPreferPreviousEvent applies rules where later writes add no value.
func shouldPreferPreviousEvent(
	previousEvent fsnotify.Event,
	incomingEvent fsnotify.Event,
	policy DeduplicationPolicy,
) bool {
	if previousEvent.Has(fsnotify.Remove) {
		return true
	}

	if policy.PreferRenameOverCreate {
		if previousEvent.Has(fsnotify.Rename) &&
			incomingEvent.Has(fsnotify.Create) {
			return true
		}
	}

	if previousEvent.Op == incomingEvent.Op {
		return true
	}

	return false
}

// FilterEventsByPathPrefix keeps only events under one path prefix.
func FilterEventsByPathPrefix(
	events []fsnotify.Event,
	pathPrefix string,
) []fsnotify.Event {
	normalizedPrefix := NormalizeEventPath(pathPrefix)
	if normalizedPrefix == "" {
		return nil
	}
	normalizedPrefixWithSlash := normalizedPrefix + "/"

	filteredEvents := make([]fsnotify.Event, 0, len(events))
	for _, event := range events {
		normalizedPath := NormalizeEventPath(event.Name)
		if normalizedPath == normalizedPrefix ||
			strings.HasPrefix(normalizedPath, normalizedPrefixWithSlash) {
			event.Name = normalizedPath
			filteredEvents = append(filteredEvents, event)
		}
	}
	return filteredEvents
}

// GroupEventsByDirectory groups events by direct parent directory.
func GroupEventsByDirectory(
	events []fsnotify.Event,
) map[string][]fsnotify.Event {
	eventsByDirectory := make(map[string][]fsnotify.Event)
	for _, event := range events {
		normalizedPath := NormalizeEventPath(event.Name)
		if normalizedPath == "" {
			continue
		}
		directory := NormalizeEventPath(filepath.Dir(normalizedPath))
		event.Name = normalizedPath
		eventsByDirectory[directory] = append(
			eventsByDirectory[directory],
			event,
		)
	}
	return eventsByDirectory
}

// BuildCanonicalPathAliasSet returns all equivalent path spellings for matching.
func BuildCanonicalPathAliasSet(path string) map[string]struct{} {
	aliasSet := make(map[string]struct{})
	normalizedPath := NormalizeEventPath(path)
	if normalizedPath == "" {
		return aliasSet
	}

	aliasSet[normalizedPath] = struct{}{}
	aliasSet[strings.ToLower(normalizedPath)] = struct{}{}
	aliasSet[strings.TrimPrefix(normalizedPath, "./")] = struct{}{}

	absolutePath, absolutePathError := filepath.Abs(normalizedPath)
	if absolutePathError == nil {
		normalizedAbsolutePath := NormalizeEventPath(absolutePath)
		aliasSet[normalizedAbsolutePath] = struct{}{}
		aliasSet[strings.ToLower(normalizedAbsolutePath)] = struct{}{}
	}
	return aliasSet
}

// MergeWatchedFiles merges matched watched-file configs with union semantics.
func MergeWatchedFiles(matches []*wave.WatchedFile) *wave.WatchedFile {
	if len(matches) == 0 {
		return nil
	}

	merged := &wave.WatchedFile{
		Pattern: matches[0].Pattern,

		RecompileGoBinary: false,
		RestartApp:        false,

		TreatAsNonGo:               true,
		RunOnChangeOnly:            true,
		SkipRebuildingNotification: true,

		OnlyRunClientDefinedRevalidateFunc: false,
	}

	allHooks := make([]wave.OnChangeHook, 0)
	for _, watchedFile := range matches {
		if watchedFile == nil {
			continue
		}
		if watchedFile.RecompileGoBinary {
			merged.RecompileGoBinary = true
		}
		if watchedFile.RestartApp {
			merged.RestartApp = true
		}
		if !watchedFile.TreatAsNonGo {
			merged.TreatAsNonGo = false
		}
		if !watchedFile.RunOnChangeOnly {
			merged.RunOnChangeOnly = false
		}
		if !watchedFile.SkipRebuildingNotification {
			merged.SkipRebuildingNotification = false
		}
		if watchedFile.OnlyRunClientDefinedRevalidateFunc {
			merged.OnlyRunClientDefinedRevalidateFunc = true
		}
		allHooks = append(allHooks, watchedFile.OnChangeHooks...)
	}

	merged.OnChangeHooks = allHooks
	merged.Sort()
	return merged
}

// PathAliasContains reports whether candidate exists in alias set.
func PathAliasContains(
	aliasSet map[string]struct{},
	candidatePath string,
) bool {
	if len(aliasSet) == 0 {
		return false
	}
	normalizedCandidate := NormalizeEventPath(candidatePath)
	if normalizedCandidate == "" {
		return false
	}
	_, found := aliasSet[normalizedCandidate]
	if found {
		return true
	}
	_, found = aliasSet[strings.ToLower(normalizedCandidate)]
	return found
}

// DeduplicateWatcherEventsByPath merges watcher events by resolved path key and
// returns deterministic path-sorted output.
func DeduplicateWatcherEventsByPath(
	events []fsnotify.Event,
) []fsnotify.Event {
	if len(events) == 0 {
		return nil
	}

	mergedEventOpsByPath := make(map[string]fsnotify.Op, len(events))
	deduplicationIndex := NewWatcherEventDeduplicationIndex()
	for _, event := range events {
		eventPathKey := NormalizeWatcherEventPathForDeduplication(event.Name)
		if existingPathKey := deduplicationIndex.ResolveExistingPathKey(
			mergedEventOpsByPath,
			eventPathKey,
		); existingPathKey != "" {
			eventPathKey = existingPathKey
		} else {
			deduplicationIndex.RecordPathKey(eventPathKey)
		}
		mergedEventOpsByPath[eventPathKey] |= event.Op
	}

	deduplicatedPaths := make([]string, 0, len(mergedEventOpsByPath))
	for eventPath := range mergedEventOpsByPath {
		deduplicatedPaths = append(deduplicatedPaths, eventPath)
	}
	sort.Strings(deduplicatedPaths)

	deduplicatedEvents := make([]fsnotify.Event, 0, len(deduplicatedPaths))
	for _, deduplicatedPath := range deduplicatedPaths {
		deduplicatedEvents = append(deduplicatedEvents, fsnotify.Event{
			Name: deduplicatedPath,
			Op:   mergedEventOpsByPath[deduplicatedPath],
		})
	}

	return deduplicatedEvents
}

// WatcherEventDeduplicationIndex caches canonical and alias probe resolution
// for one deduplication pass.
type WatcherEventDeduplicationIndex struct {
	// CanonicalPathToPathKey maps canonical absolute paths to first-seen keys.
	CanonicalPathToPathKey map[string]string
	// MissingFileAliasKeyToPathKey maps missing-file alias keys to first-seen
	// path keys.
	MissingFileAliasKeyToPathKey map[string]string
	// CanonicalPathByAbsolutePath memoizes canonicalization per absolute path.
	CanonicalPathByAbsolutePath map[string]string
	// MissingAliasKeyByAbsolutePath memoizes missing-file alias keys per path.
	MissingAliasKeyByAbsolutePath map[string]string
}

// NewWatcherEventDeduplicationIndex returns an empty dedup probe index.
func NewWatcherEventDeduplicationIndex() *WatcherEventDeduplicationIndex {
	return &WatcherEventDeduplicationIndex{
		CanonicalPathToPathKey:        make(map[string]string),
		MissingFileAliasKeyToPathKey:  make(map[string]string),
		CanonicalPathByAbsolutePath:   make(map[string]string),
		MissingAliasKeyByAbsolutePath: make(map[string]string),
	}
}

// ResolveExistingPathKey returns an existing path key for normalizedEventPath
// when canonical or alias probes identify a previously-seen key.
func (index *WatcherEventDeduplicationIndex) ResolveExistingPathKey(
	mergedEventOpsByPath map[string]fsnotify.Op,
	normalizedEventPath string,
) string {
	if normalizedEventPath == "" {
		return ""
	}

	if _, alreadyExists := mergedEventOpsByPath[normalizedEventPath]; alreadyExists {
		return normalizedEventPath
	}

	if !IsAbsolutePathForWatcherEventDeduplication(normalizedEventPath) {
		return ""
	}

	return index.resolveExistingPathKeyFromCanonicalAndMissingAliasProbes(
		normalizedEventPath,
	)
}

// RecordPathKey records pathKey for canonical and alias probes on first-seen
// basis.
func (index *WatcherEventDeduplicationIndex) RecordPathKey(pathKey string) {
	if !IsAbsolutePathForWatcherEventDeduplication(pathKey) {
		return
	}

	index.recordPathKeyForCanonicalAndMissingAliasProbes(pathKey)
}

// MissingAliasKeyResolutionEmptySentinel caches negative alias-key probes.
const MissingAliasKeyResolutionEmptySentinel = "\x00"

// ResolveCanonicalPathForAbsolutePath canonicalizes and memoizes absolutePath.
func (index *WatcherEventDeduplicationIndex) ResolveCanonicalPathForAbsolutePath(
	absolutePath string,
) string {
	if !IsAbsolutePathForWatcherEventDeduplication(absolutePath) {
		return ""
	}

	if canonicalPath, exists := index.CanonicalPathByAbsolutePath[absolutePath]; exists {
		return canonicalPath
	}

	canonicalPath := canonicalizeAbsolutePathForWatcherEventDeduplication(
		absolutePath,
	)
	if canonicalPath == "" {
		return ""
	}
	index.CanonicalPathByAbsolutePath[absolutePath] = canonicalPath
	return canonicalPath
}

// ResolveMissingAliasKeyForAbsolutePath resolves and memoizes missing-file
// alias keys for absolutePath.
func (index *WatcherEventDeduplicationIndex) ResolveMissingAliasKeyForAbsolutePath(
	absolutePath string,
) string {
	if !IsAbsolutePathForWatcherEventDeduplication(absolutePath) {
		return ""
	}

	if cachedAliasKey, exists := index.MissingAliasKeyByAbsolutePath[absolutePath]; exists {
		if cachedAliasKey == MissingAliasKeyResolutionEmptySentinel {
			return ""
		}
		return cachedAliasKey
	}

	missingAliasKey := MissingFileAliasKeyForWatcherEventDeduplication(
		absolutePath,
	)
	if missingAliasKey == "" {
		index.MissingAliasKeyByAbsolutePath[absolutePath] = MissingAliasKeyResolutionEmptySentinel
		return ""
	}

	index.MissingAliasKeyByAbsolutePath[absolutePath] = missingAliasKey
	return missingAliasKey
}

// resolveExistingPathKeyFromCanonicalAndMissingAliasProbes checks canonical and
// missing-file alias probes in deterministic order.
func (index *WatcherEventDeduplicationIndex) resolveExistingPathKeyFromCanonicalAndMissingAliasProbes(
	absolutePath string,
) string {
	if !IsAbsolutePathForWatcherEventDeduplication(absolutePath) {
		return ""
	}

	if existingPathKey := index.resolveExistingPathKeyFromCanonicalPathProbe(absolutePath); existingPathKey != "" {
		return existingPathKey
	}

	return index.resolveExistingPathKeyFromMissingAliasProbe(absolutePath)
}

// resolveExistingPathKeyFromCanonicalPathProbe resolves a path key from the
// canonical path probe lookup.
func (index *WatcherEventDeduplicationIndex) resolveExistingPathKeyFromCanonicalPathProbe(
	absolutePath string,
) string {
	canonicalPath := index.ResolveCanonicalPathForAbsolutePath(absolutePath)
	return lookupFirstSeenPathKeyByProbeKey(
		index.CanonicalPathToPathKey,
		canonicalPath,
	)
}

// resolveExistingPathKeyFromMissingAliasProbe resolves a path key from the
// missing-file alias probe lookup.
func (index *WatcherEventDeduplicationIndex) resolveExistingPathKeyFromMissingAliasProbe(
	absolutePath string,
) string {
	missingAliasKey := index.ResolveMissingAliasKeyForAbsolutePath(absolutePath)
	return lookupFirstSeenPathKeyByProbeKey(
		index.MissingFileAliasKeyToPathKey,
		missingAliasKey,
	)
}

// recordPathKeyForCanonicalAndMissingAliasProbes records first-seen mappings
// for canonical and missing-file alias probe lookups.
func (index *WatcherEventDeduplicationIndex) recordPathKeyForCanonicalAndMissingAliasProbes(
	pathKey string,
) {
	if !IsAbsolutePathForWatcherEventDeduplication(pathKey) {
		return
	}

	canonicalPath := index.ResolveCanonicalPathForAbsolutePath(pathKey)
	recordFirstSeenPathKeyByProbeKeyIfMissing(
		index.CanonicalPathToPathKey,
		canonicalPath,
		pathKey,
	)

	missingAliasKey := index.ResolveMissingAliasKeyForAbsolutePath(pathKey)
	recordFirstSeenPathKeyByProbeKeyIfMissing(
		index.MissingFileAliasKeyToPathKey,
		missingAliasKey,
		pathKey,
	)
}

// NormalizeWatcherEventPathForDeduplication trims and cleans path shapes used
// as primary dedup keys.
func NormalizeWatcherEventPathForDeduplication(
	eventPath string,
) string {
	cleanedEventPath := wavecore.TrimAndCleanPath(eventPath)
	if cleanedEventPath == "" {
		return ""
	}

	if IsAbsolutePathForWatcherEventDeduplication(cleanedEventPath) {
		return wavecore.Absolute(cleanedEventPath)
	}

	return cleanedEventPath
}

// canonicalizeAbsolutePathForWatcherEventDeduplication resolves a canonical path
// identity for location comparisons.
func canonicalizeAbsolutePathForWatcherEventDeduplication(
	absolutePath string,
) string {
	return wavecore.CanonicalizePathForLocationComparison(absolutePath)
}

// MissingFileAliasKeyForWatcherEventDeduplication resolves a stable alias key
// for missing absolute file paths (canonical-parent + base-name).
func MissingFileAliasKeyForWatcherEventDeduplication(
	absolutePath string,
) string {
	if !IsAbsolutePathForWatcherEventDeduplication(absolutePath) {
		return ""
	}

	baseName := filepath.Base(absolutePath)
	if baseName == "" || baseName == "." || baseName == string(filepath.Separator) {
		return ""
	}

	parentDirectoryPath := wavecore.AbsoluteDirectory(absolutePath)
	if parentDirectoryPath == "" {
		return ""
	}
	canonicalParentDirectoryPath := canonicalizeAbsolutePathForWatcherEventDeduplication(
		parentDirectoryPath,
	)
	if canonicalParentDirectoryPath == "" {
		return ""
	}

	return canonicalParentDirectoryPath + "\x00" + baseName
}

// IsAbsolutePathForWatcherEventDeduplication reports whether path is an
// absolute non-empty path.
func IsAbsolutePathForWatcherEventDeduplication(path string) bool {
	return path != "" && filepath.IsAbs(path)
}

// lookupFirstSeenPathKeyByProbeKey reads from one probe lookup map
// (canonical-path or missing-alias lookup) and returns the first-seen path key
// associated with probeKey.
func lookupFirstSeenPathKeyByProbeKey(
	firstSeenPathKeyByProbeKey map[string]string,
	probeKey string,
) string {
	if firstSeenPathKeyByProbeKey == nil || probeKey == "" {
		return ""
	}
	return firstSeenPathKeyByProbeKey[probeKey]
}

// recordFirstSeenPathKeyByProbeKeyIfMissing preserves the first-seen path key
// for a probeKey. This prevents later alias paths from replacing the earlier
// path-key label and keeps dedup output stable for one watcher batch.
func recordFirstSeenPathKeyByProbeKeyIfMissing(
	firstSeenPathKeyByProbeKey map[string]string,
	probeKey string,
	pathKey string,
) {
	if firstSeenPathKeyByProbeKey == nil || probeKey == "" || pathKey == "" {
		return
	}
	if _, exists := firstSeenPathKeyByProbeKey[probeKey]; exists {
		return
	}
	firstSeenPathKeyByProbeKey[probeKey] = pathKey
}
