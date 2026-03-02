package runloop_test

import (
	"github.com/vormadev/vorma/wave/buildtime/internal/devserver/eventpipeline"
	"github.com/vormadev/vorma/wave/buildtime/internal/devserver/runloop"
	"github.com/vormadev/vorma/wave/buildtime/internal/watch"
)

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
