package tooling

import "sync"

type restartIntentAccumulator struct {
	mu                   sync.Mutex
	restartRequests      chan restartRequest
	waitingForBuildRetry bool
}

func newRestartIntentAccumulator(
	restartRequests chan restartRequest,
) *restartIntentAccumulator {
	if restartRequests == nil {
		restartRequests = make(chan restartRequest, 1)
	}
	return &restartIntentAccumulator{
		restartRequests: restartRequests,
	}
}

func (accumulator *restartIntentAccumulator) setWaitingForBuildRetry(
	waitingForBuildRetry bool,
) {
	if accumulator == nil {
		return
	}

	accumulator.mu.Lock()
	accumulator.waitingForBuildRetry = waitingForBuildRetry
	accumulator.mu.Unlock()
}

func (accumulator *restartIntentAccumulator) queueRestartRequest(
	restartRequestForQueue restartRequest,
) {
	if accumulator == nil {
		return
	}

	accumulator.mu.Lock()
	defer accumulator.mu.Unlock()

	incomingRequest := normalizeRestartRequest(restartRequestForQueue)
	if accumulator.waitingForBuildRetry {
		// During build-retry wait, first queued request deterministically
		// controls next pass. Do not merge/upgrade while waiting.
		_ = tryEnqueueRestartRequest(accumulator.restartRequests, incomingRequest)
		return
	}

	if tryEnqueueRestartRequest(accumulator.restartRequests, incomingRequest) {
		return
	}

	pendingRequest, hasPendingRequest := tryDequeueRestartRequest(
		accumulator.restartRequests,
	)
	if !hasPendingRequest {
		_ = tryEnqueueRestartRequest(accumulator.restartRequests, incomingRequest)
		return
	}

	queuedRequest := resolveQueuedRestartRequest(
		&pendingRequest,
		incomingRequest,
	)
	_ = tryEnqueueRestartRequest(accumulator.restartRequests, queuedRequest)
}

func (accumulator *restartIntentAccumulator) consumePendingRestartRequest() (restartRequest, bool) {
	if accumulator == nil {
		return restartRequest{}, false
	}

	accumulator.mu.Lock()
	defer accumulator.mu.Unlock()

	return tryDequeueRestartRequest(accumulator.restartRequests)
}

func (accumulator *restartIntentAccumulator) consumeRestartRequestBlocking() restartRequest {
	if accumulator == nil {
		return restartRequest{}
	}

	if pendingRequest, hasPendingRequest := accumulator.consumePendingRestartRequest(); hasPendingRequest {
		return normalizeRestartRequest(pendingRequest)
	}

	return normalizeRestartRequest(<-accumulator.restartRequests)
}

func (s *server) getOrCreateRestartIntentAccumulator() *restartIntentAccumulator {
	if s == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.restartIntents == nil {
		s.restartIntents = newRestartIntentAccumulator(make(chan restartRequest, 1))
	}
	return s.restartIntents
}

func (s *server) setWaitingForBuildRetry(waitingForBuildRetry bool) {
	if s == nil {
		return
	}

	accumulator := s.getOrCreateRestartIntentAccumulator()
	if accumulator != nil {
		accumulator.setWaitingForBuildRetry(waitingForBuildRetry)
	}
}

func (s *server) queueRestartRequest(
	restartRequestForQueue restartRequest,
) {
	accumulator := s.getOrCreateRestartIntentAccumulator()
	if accumulator == nil {
		return
	}
	accumulator.queueRestartRequest(restartRequestForQueue)
}

func (s *server) consumePendingRestartRequest() (restartRequest, bool) {
	accumulator := s.getOrCreateRestartIntentAccumulator()
	if accumulator == nil {
		return restartRequest{}, false
	}
	return accumulator.consumePendingRestartRequest()
}

func (s *server) consumeRestartRequestBlocking() restartRequest {
	accumulator := s.getOrCreateRestartIntentAccumulator()
	if accumulator == nil {
		return restartRequest{}
	}
	return accumulator.consumeRestartRequestBlocking()
}

// triggerRestart triggers a restart with Go recompilation
func (s *server) triggerRestart() {
	s.triggerRestartWithOpts(true, false)
}

// triggerRestartNoGo triggers a restart without Go recompilation
func (s *server) triggerRestartNoGo() {
	s.triggerRestartWithOpts(false, false)
}

// triggerConfigRestart triggers a restart due to config file change.
// Config restarts always recompile Go and take precedence over other pending restarts.
func (s *server) triggerConfigRestart() {
	s.triggerRestartWithOpts(true, true)
}

// triggerRestartWithOpts handles restart requests with upgrade semantics.
func (s *server) triggerRestartWithOpts(recompileGo bool, isConfigRestart bool) {
	incomingRequest := normalizeRestartRequest(restartRequest{
		recompileGo:     recompileGo,
		isConfigRestart: isConfigRestart,
	})
	s.queueRestartRequest(incomingRequest)
}

func normalizeRestartRequest(request restartRequest) restartRequest {
	if request.isConfigRestart {
		request.recompileGo = true
	}
	return request
}

func resolveQueuedRestartRequest(
	pendingRequest *restartRequest,
	incomingRequest restartRequest,
) restartRequest {
	normalizedIncomingRequest := normalizeRestartRequest(incomingRequest)
	if pendingRequest == nil {
		return normalizedIncomingRequest
	}

	normalizedPendingRequest := normalizeRestartRequest(*pendingRequest)
	return mergeRestartRequests(normalizedPendingRequest, normalizedIncomingRequest)
}

func tryEnqueueRestartRequest(
	restartRequests chan restartRequest,
	request restartRequest,
) bool {
	select {
	case restartRequests <- request:
		return true
	default:
		return false
	}
}

func tryDequeueRestartRequest(
	restartRequests chan restartRequest,
) (restartRequest, bool) {
	select {
	case pendingRequest := <-restartRequests:
		return pendingRequest, true
	default:
		return restartRequest{}, false
	}
}

func mergeRestartRequests(
	pendingRequest restartRequest,
	incomingRequest restartRequest,
) restartRequest {
	if pendingRequest.isConfigRestart || incomingRequest.isConfigRestart {
		return restartRequest{
			recompileGo:     true,
			isConfigRestart: true,
		}
	}

	return restartRequest{
		recompileGo:     pendingRequest.recompileGo || incomingRequest.recompileGo,
		isConfigRestart: false,
	}
}
