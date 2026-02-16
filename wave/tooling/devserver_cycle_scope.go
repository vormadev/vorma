package tooling

import (
	"context"
	"sync"
)

type runCycleScope struct {
	cycleID uint64

	executionContext       context.Context
	cancelExecutionContext context.CancelFunc

	asyncWorkGroup sync.WaitGroup
}

func newRunCycleScope(
	cycleID uint64,
) *runCycleScope {
	executionContext, cancelExecutionContext := context.WithCancel(
		context.Background(),
	)
	return &runCycleScope{
		cycleID:                cycleID,
		executionContext:       executionContext,
		cancelExecutionContext: cancelExecutionContext,
	}
}

func (scope *runCycleScope) launchAsyncWork(
	runAsyncWork func(context.Context),
) {
	if scope == nil || runAsyncWork == nil {
		return
	}

	scope.asyncWorkGroup.Add(1)
	go func() {
		defer scope.asyncWorkGroup.Done()
		runAsyncWork(scope.executionContext)
	}()
}

func (scope *runCycleScope) cancelAndJoin() {
	if scope == nil {
		return
	}

	if scope.cancelExecutionContext != nil {
		scope.cancelExecutionContext()
	}
	scope.asyncWorkGroup.Wait()
}

func (s *server) startRunCycleScope() *runCycleScope {
	if s == nil {
		return nil
	}

	s.mu.Lock()
	s.nextRunCycleID++
	cycleScope := newRunCycleScope(s.nextRunCycleID)
	s.currentRunCycleScope = cycleScope
	s.mu.Unlock()

	return cycleScope
}

func (s *server) getCurrentRunCycleScope() *runCycleScope {
	if s == nil {
		return nil
	}

	s.mu.Lock()
	cycleScope := s.currentRunCycleScope
	s.mu.Unlock()
	return cycleScope
}

func (s *server) currentRunCycleContextOrBackground() context.Context {
	currentRunCycleScope := s.getCurrentRunCycleScope()
	if currentRunCycleScope == nil || currentRunCycleScope.executionContext == nil {
		return context.Background()
	}
	return currentRunCycleScope.executionContext
}

func (s *server) cancelAndJoinCurrentRunCycleScope() {
	if s == nil {
		return
	}

	s.mu.Lock()
	currentRunCycleScope := s.currentRunCycleScope
	s.currentRunCycleScope = nil
	s.mu.Unlock()

	if currentRunCycleScope != nil {
		currentRunCycleScope.cancelAndJoin()
	}
}

func (s *server) launchRunCycleScopedAsyncWorkOrDetached(
	runAsyncWork func(context.Context),
) {
	if runAsyncWork == nil {
		return
	}

	currentRunCycleScope := s.getCurrentRunCycleScope()
	if currentRunCycleScope != nil {
		currentRunCycleScope.launchAsyncWork(runAsyncWork)
		return
	}

	go runAsyncWork(context.Background())
}
