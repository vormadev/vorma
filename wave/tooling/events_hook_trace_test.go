package tooling

import (
	"bytes"
	"errors"
	"github.com/vormadev/vorma/wave/tooling/devserver"
	"github.com/vormadev/vorma/wave/tooling/watch"
	"log/slog"
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave"
)

func TestRunSequentialHookStageForEligibleEventsWithErrors_LogsTraceFields(t *testing.T) {
	var hookStageLogBuffer bytes.Buffer
	s := &devserver.Server{
		Log: slog.New(slog.NewTextHandler(&hookStageLogBuffer, nil)),
	}
	s.SetCurrentWatcherExecutionTraceContext(
		devserver.WatcherExecutionTraceContext{CycleID: 7, BatchID: 3},
	)
	defer s.ClearCurrentWatcherExecutionTraceContext()

	_, stageExecutionErrors := s.RunSequentialHookStageForEligibleEventsWithErrors(
		[]devserver.EventWithHooks{
			{
				Classified: devserver.ClassifiedEvent{
					Event: waveEvent("changed.txt"),
				},
			},
		},
		nil,
		func(devserver.EventWithHooks, *watch.Watcher) ([]wave.RefreshAction, error) {
			return nil, errors.New("synthetic stage failure")
		},
		"Pre-hook execution failed",
	)

	if len(stageExecutionErrors) != 1 {
		t.Fatalf("expected one stage execution error, got %#v", stageExecutionErrors)
	}

	hookStageLogOutput := hookStageLogBuffer.String()
	if !strings.Contains(hookStageLogOutput, "cycle_id=7") {
		t.Fatalf("expected hook-stage log to include cycle_id=7, got %q", hookStageLogOutput)
	}
	if !strings.Contains(hookStageLogOutput, "batch_id=3") {
		t.Fatalf("expected hook-stage log to include batch_id=3, got %q", hookStageLogOutput)
	}
}
