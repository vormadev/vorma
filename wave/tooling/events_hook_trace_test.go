package tooling

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave"
)

func TestRunSequentialHookStageForEligibleEventsWithErrors_LogsTraceFields(t *testing.T) {
	var hookStageLogBuffer bytes.Buffer
	s := &server{
		log: slog.New(slog.NewTextHandler(&hookStageLogBuffer, nil)),
	}
	s.setCurrentWatcherExecutionTraceContext(
		watcherExecutionTraceContext{cycleID: 7, batchID: 3},
	)
	defer s.clearCurrentWatcherExecutionTraceContext()

	_, stageExecutionErrors := s.runSequentialHookStageForEligibleEventsWithErrors(
		[]eventWithHooks{
			{
				classified: classifiedEvent{
					event: waveEvent("changed.txt"),
				},
			},
		},
		nil,
		func(eventWithHooks, *watcher) ([]wave.RefreshAction, error) {
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
