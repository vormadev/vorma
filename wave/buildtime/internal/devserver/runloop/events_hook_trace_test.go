package runloop_test

import (
	"bytes"
	"errors"
	"github.com/vormadev/vorma/wave/wavewatch"
	"log/slog"
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave/buildtime/internal/devserver/eventpipeline"
	"github.com/vormadev/vorma/wave/buildtime/internal/devserver/runloop"
	"github.com/vormadev/vorma/wave/buildtime/internal/watch"
)

func TestRunSequentialHookStageForEligibleEventsWithErrors_LogsTraceFields(
	t *testing.T,
) {
	var hookStageLogBuffer bytes.Buffer
	engine := runloop.New(
		runloop.Dependencies{
			Log: slog.New(slog.NewTextHandler(&hookStageLogBuffer, nil)),
			GetCurrentWatcherExecutionTraceContext: func() runloop.WatcherExecutionTraceContext {
				return runloop.WatcherExecutionTraceContext{
					CycleID: 7,
					BatchID: 3,
				}
			},
		},
	)

	_, stageExecutionErrors := engine.RunSequentialHookStageForEligibleEventsWithErrors(
		[]eventpipeline.EventWithHooks{
			{
				Classified: eventpipeline.ClassifiedEvent{
					Event: waveEvent("changed.txt"),
				},
			},
		},
		nil,
		func(eventpipeline.EventWithHooks, *watch.Watcher) ([]wavewatch.RefreshAction, error) {
			return nil, errors.New("synthetic stage failure")
		},
		"Pre-hook execution failed",
	)

	if len(stageExecutionErrors) != 1 {
		t.Fatalf(
			"expected one stage execution error, got %#v",
			stageExecutionErrors,
		)
	}

	hookStageLogOutput := hookStageLogBuffer.String()
	if !strings.Contains(hookStageLogOutput, "cycle_id=7") {
		t.Fatalf(
			"expected hook-stage log to include cycle_id=7, got %q",
			hookStageLogOutput,
		)
	}
	if !strings.Contains(hookStageLogOutput, "batch_id=3") {
		t.Fatalf(
			"expected hook-stage log to include batch_id=3, got %q",
			hookStageLogOutput,
		)
	}
}
