package tooling

type watcherExecutionTraceContext struct {
	cycleID uint64
	batchID uint64
}

func (s *server) deriveWatcherExecutionTraceContext() watcherExecutionTraceContext {
	if s == nil {
		return watcherExecutionTraceContext{}
	}

	s.mu.Lock()
	s.nextWatcherBatchID++
	nextBatchID := s.nextWatcherBatchID
	currentRunCycleScope := s.currentRunCycleScope
	s.mu.Unlock()

	currentCycleID := uint64(0)
	if currentRunCycleScope != nil {
		currentCycleID = currentRunCycleScope.cycleID
	}

	return watcherExecutionTraceContext{
		cycleID: currentCycleID,
		batchID: nextBatchID,
	}
}

func (s *server) setCurrentWatcherExecutionTraceContext(
	traceContextForWatcherExecution watcherExecutionTraceContext,
) {
	if s == nil {
		return
	}

	s.mu.Lock()
	s.currentWatcherExecutionTraceContext = traceContextForWatcherExecution
	s.mu.Unlock()
}

func (s *server) clearCurrentWatcherExecutionTraceContext() {
	if s == nil {
		return
	}

	s.mu.Lock()
	s.currentWatcherExecutionTraceContext = watcherExecutionTraceContext{}
	s.mu.Unlock()
}

func (s *server) getCurrentWatcherExecutionTraceContext() watcherExecutionTraceContext {
	if s == nil {
		return watcherExecutionTraceContext{}
	}

	s.mu.Lock()
	traceContextForWatcherExecution := s.currentWatcherExecutionTraceContext
	s.mu.Unlock()
	return traceContextForWatcherExecution
}
