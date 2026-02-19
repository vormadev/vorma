package tooling

import (
	"github.com/vormadev/vorma/wave/tooling/devserver"
	"testing"

	"github.com/fsnotify/fsnotify"
)

func TestDeriveHookStageExecutionDescriptors_SkipsDuplicateHooksAndPreservesOrder(t *testing.T) {
	eventsWithHooks := []devserver.EventWithHooks{
		{
			Classified:         devserver.ClassifiedEvent{Event: fsnotify.Event{Name: "a.go"}},
			SkipDuplicateHooks: false,
		},
		{
			Classified:         devserver.ClassifiedEvent{Event: fsnotify.Event{Name: "a.go"}},
			SkipDuplicateHooks: true,
		},
		{
			Classified:         devserver.ClassifiedEvent{Event: fsnotify.Event{Name: "b.css"}},
			SkipDuplicateHooks: false,
		},
	}

	descriptors := devserver.DeriveHookStageExecutionDescriptors(eventsWithHooks)
	if len(descriptors) != 2 {
		t.Fatalf("expected 2 descriptors, got %d", len(descriptors))
	}

	if descriptors[0].EventIndex != 0 {
		t.Fatalf("expected first descriptor event index 0, got %d", descriptors[0].EventIndex)
	}
	if descriptors[0].EventWithHooks.Classified.Event.Name != "a.go" {
		t.Fatalf("expected first descriptor for a.go, got %q", descriptors[0].EventWithHooks.Classified.Event.Name)
	}

	if descriptors[1].EventIndex != 2 {
		t.Fatalf("expected second descriptor event index 2, got %d", descriptors[1].EventIndex)
	}
	if descriptors[1].EventWithHooks.Classified.Event.Name != "b.css" {
		t.Fatalf("expected second descriptor for b.css, got %q", descriptors[1].EventWithHooks.Classified.Event.Name)
	}
}

func TestDeriveHookStageExecutionDescriptors_EmptyInputReturnsNil(t *testing.T) {
	descriptors := devserver.DeriveHookStageExecutionDescriptors(nil)
	if descriptors != nil {
		t.Fatalf("expected nil descriptors for nil input, got %#v", descriptors)
	}

	descriptors = devserver.DeriveHookStageExecutionDescriptors([]devserver.EventWithHooks{})
	if descriptors != nil {
		t.Fatalf("expected nil descriptors for empty input, got %#v", descriptors)
	}
}
