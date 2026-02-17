package tooling

func (w *watcher) setupPatterns() error {
	watcherPlanForSetup, buildWatcherPlanError := w.buildWatcherPlan()
	if buildWatcherPlanError != nil {
		return buildWatcherPlanError
	}
	w.applyWatcherPlan(watcherPlanForSetup)
	return nil
}
