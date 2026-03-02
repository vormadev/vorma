package hooks_test

import (
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave/buildtime/internal/devserver/eventpipeline"
	"github.com/vormadev/vorma/wave/buildtime/internal/devserver/hooks"
)

func TestDeriveHookStageExecutionDescriptors_SkipsDuplicateHooksAndPreservesOrder(
	t *testing.T,
) {
	eventsWithHooks := []eventpipeline.EventWithHooks{
		{
			Classified: eventpipeline.ClassifiedEvent{
				Event: fsnotify.Event{Name: "a.go"},
			},
			SkipDuplicateHooks: false,
		},
		{
			Classified: eventpipeline.ClassifiedEvent{
				Event: fsnotify.Event{Name: "a.go"},
			},
			SkipDuplicateHooks: true,
		},
		{
			Classified: eventpipeline.ClassifiedEvent{
				Event: fsnotify.Event{Name: "b.css"},
			},
			SkipDuplicateHooks: false,
		},
	}

	descriptors := hooks.DeriveHookStageExecutionDescriptors(eventsWithHooks)
	if len(descriptors) != 2 {
		t.Fatalf("expected 2 descriptors, got %d", len(descriptors))
	}

	if descriptors[0].DescriptorIndex != 0 {
		t.Fatalf(
			"expected first descriptor index 0, got %d",
			descriptors[0].DescriptorIndex,
		)
	}
	if descriptors[0].EventWithHooks.Classified.Event.Name != "a.go" {
		t.Fatalf(
			"expected first descriptor for a.go, got %q",
			descriptors[0].EventWithHooks.Classified.Event.Name,
		)
	}

	if descriptors[1].DescriptorIndex != 2 {
		t.Fatalf(
			"expected second descriptor index 2, got %d",
			descriptors[1].DescriptorIndex,
		)
	}
	if descriptors[1].EventWithHooks.Classified.Event.Name != "b.css" {
		t.Fatalf(
			"expected second descriptor for b.css, got %q",
			descriptors[1].EventWithHooks.Classified.Event.Name,
		)
	}
}

func TestDeriveHookStageExecutionDescriptors_EmptyInputReturnsNil(
	t *testing.T,
) {
	descriptors := hooks.DeriveHookStageExecutionDescriptors(nil)
	if descriptors != nil {
		t.Fatalf("expected nil descriptors for nil input, got %#v", descriptors)
	}

	descriptors = hooks.DeriveHookStageExecutionDescriptors(
		[]eventpipeline.EventWithHooks{},
	)
	if descriptors != nil {
		t.Fatalf(
			"expected nil descriptors for empty input, got %#v",
			descriptors,
		)
	}
}
