package tooling

import "testing"

func runEventsWithDerivedExecutionPlan(
	t *testing.T,
	serverForTest *server,
	eventsWithHooks []eventWithHooks,
	work *workSet,
	watcher *Watcher,
) {
	t.Helper()

	executionPlan := &eventExecutionPlan{
		eventsWithHooks:  eventsWithHooks,
		appStopStrategy:  resolveAppStopStrategy(eventsWithHooks),
		runImplicitBuild: shouldRunImplicitBuildForEvents(eventsWithHooks),
	}
	serverForTest.executeEventExecutionPlan(executionPlan, work, watcher)
}
