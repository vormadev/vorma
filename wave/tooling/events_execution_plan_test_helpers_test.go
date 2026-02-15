package tooling

import "testing"

func runEventsWithDerivedExecutionPlan(
	t *testing.T,
	serverForTest *server,
	eventsWithHooks []eventWithHooks,
	work *workSet,
	watcher *watcher,
) {
	t.Helper()

	behavioralDecision := deriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
		eventsWithHooks,
	)
	serverForTest.executeEventExecutionPlan(
		eventsWithHooks,
		behavioralDecision,
		work,
		watcher,
	)
}
