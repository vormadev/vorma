package tooling

import (
	"github.com/vormadev/vorma/wave/tooling/devserver"
	"testing"

	"github.com/vormadev/vorma/wave/tooling/watch"
)

func runEventsWithDerivedExecutionPlan(
	t *testing.T,
	serverForTest *devserver.Server,
	eventsWithHooks []devserver.EventWithHooks,
	work *devserver.WorkSet,
	watcher *watch.Watcher,
) {
	t.Helper()

	behavioralDecision := devserver.DeriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
		eventsWithHooks,
	)
	serverForTest.ExecuteEventExecutionPlan(
		eventsWithHooks,
		behavioralDecision,
		work,
		watcher,
	)
}
