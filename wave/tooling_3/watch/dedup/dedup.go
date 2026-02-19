package dedup

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/fsnotify/fsnotify"
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

	for eventIndex := range rawEvents {
		rawEvent := rawEvents[eventIndex]
		normalizedPath := NormalizeEventPath(rawEvent.Name)
		if normalizedPath == "" {
			continue
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
