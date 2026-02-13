package tooling

func (s *server) executeAppStopStrategy(
	appStopStrategyForExecution appStopStrategy,
) {
	switch appStopStrategyForExecution {
	case appStopStrategySingleEventHardReload:
		s.log.Info("Terminating running app")
		if err := s.stopApp(); err != nil {
			s.log.Error("Failed to terminate app", "error", err)
		}

	case appStopStrategyBatchHardReload:
		s.log.Info("Stopping app for batch rebuild")
		if err := s.stopApp(); err != nil {
			s.log.Error("Failed to stop app", "error", err)
		}

	case appStopStrategyNone:
	}
}
