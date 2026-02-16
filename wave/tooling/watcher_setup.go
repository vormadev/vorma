package tooling

func (w *watcher) setupPatterns() {
	watcherPlanForSetup := w.buildWatcherPlan()
	w.applyWatcherPlan(watcherPlanForSetup)
}
