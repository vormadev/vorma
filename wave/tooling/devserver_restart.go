package tooling

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
	s.restartChMu.Lock()
	defer s.restartChMu.Unlock()

	incomingRequest := normalizeRestartRequest(restartRequest{
		recompileGo:     recompileGo,
		isConfigRestart: isConfigRestart,
	})

	// Try to enqueue directly when no restart is pending.
	if tryEnqueueRestartRequest(s.restartCh, incomingRequest) {
		return
	}

	// Merge with the currently pending request.
	pendingRequest, hasPendingRequest := tryDequeueRestartRequest(s.restartCh)
	if !hasPendingRequest {
		// Channel became empty after the initial check (consumer took pending).
		// Best effort enqueue of current request.
		_ = tryEnqueueRestartRequest(s.restartCh, incomingRequest)
		return
	}

	queuedRequest := resolveQueuedRestartRequest(&pendingRequest, incomingRequest)
	_ = tryEnqueueRestartRequest(s.restartCh, queuedRequest)
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
