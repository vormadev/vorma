package tooling

import (
	"testing"

	"github.com/fsnotify/fsnotify"
)

func TestDeriveHookStageExecutionDescriptors_SkipsDuplicateHooksAndPreservesOrder(t *testing.T) {
	eventsWithHooks := []eventWithHooks{
		{
			classified:         classifiedEvent{event: fsnotify.Event{Name: "a.go"}},
			skipDuplicateHooks: false,
		},
		{
			classified:         classifiedEvent{event: fsnotify.Event{Name: "a.go"}},
			skipDuplicateHooks: true,
		},
		{
			classified:         classifiedEvent{event: fsnotify.Event{Name: "b.css"}},
			skipDuplicateHooks: false,
		},
	}

	descriptors := deriveHookStageExecutionDescriptors(eventsWithHooks)
	if len(descriptors) != 2 {
		t.Fatalf("expected 2 descriptors, got %d", len(descriptors))
	}

	if descriptors[0].eventIndex != 0 {
		t.Fatalf("expected first descriptor event index 0, got %d", descriptors[0].eventIndex)
	}
	if descriptors[0].eventWithHooks.classified.event.Name != "a.go" {
		t.Fatalf("expected first descriptor for a.go, got %q", descriptors[0].eventWithHooks.classified.event.Name)
	}

	if descriptors[1].eventIndex != 2 {
		t.Fatalf("expected second descriptor event index 2, got %d", descriptors[1].eventIndex)
	}
	if descriptors[1].eventWithHooks.classified.event.Name != "b.css" {
		t.Fatalf("expected second descriptor for b.css, got %q", descriptors[1].eventWithHooks.classified.event.Name)
	}
}

func TestDeriveHookStageExecutionDescriptors_EmptyInputReturnsNil(t *testing.T) {
	descriptors := deriveHookStageExecutionDescriptors(nil)
	if descriptors != nil {
		t.Fatalf("expected nil descriptors for nil input, got %#v", descriptors)
	}

	descriptors = deriveHookStageExecutionDescriptors([]eventWithHooks{})
	if descriptors != nil {
		t.Fatalf("expected nil descriptors for empty input, got %#v", descriptors)
	}
}
