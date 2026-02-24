package runloop_test

import (
	"github.com/vormadev/vorma/wave/wavedev/devserver/internal/eventpipeline"
	"github.com/vormadev/vorma/wave/wavedev/devserver/internal/runloop"
	"github.com/vormadev/vorma/wave/wavedev/internal/watch"
)

type staticAssetDirsForTests = struct {
	Private string `json:"Private"`
	Public  string `json:"Public"`
}

type cssEntryFilesForTests = struct {
	Critical    string `json:"Critical,omitempty"`
	NonCritical string `json:"NonCritical,omitempty"`
}

func runEventsWithDerivedExecutionPlan(
	engine *runloop.Engine,
	events []eventpipeline.EventWithHooks,
	work *eventpipeline.WorkSet,
	watcherForTest *watch.Watcher,
) {
	if engine == nil {
		return
	}
	behavioralDecision := eventpipeline.DeriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
		events,
	)
	engine.ExecuteEventExecutionPlan(
		events,
		behavioralDecision,
		work,
		watcherForTest,
	)
}
