package tooling

import "golang.org/x/sync/errgroup"

type staticFileProcessingExecutionMode int

const (
	staticFileProcessingExecutionModeNone staticFileProcessingExecutionMode = iota
	staticFileProcessingExecutionModeFullScan
	staticFileProcessingExecutionModeChangedPaths
)

type staticFileProcessingExecutionDecision struct {
	mode             staticFileProcessingExecutionMode
	changedFilePaths []string
}

func (decision staticFileProcessingExecutionDecision) shouldProcess() bool {
	return decision.mode != staticFileProcessingExecutionModeNone
}

type buildPhaseExecutionDecision struct {
	compileGo                   bool
	publicStaticProcessing      staticFileProcessingExecutionDecision
	privateStaticProcessing     staticFileProcessingExecutionDecision
	buildCriticalCSS            bool
	buildNormalCSS              bool
	writeFrameworkPublicFileMap bool
}

func deriveStaticFileProcessingExecutionDecision(
	shouldProcess bool,
	changedFilePaths []string,
) staticFileProcessingExecutionDecision {
	if !shouldProcess {
		return staticFileProcessingExecutionDecision{}
	}
	if len(changedFilePaths) == 0 {
		return staticFileProcessingExecutionDecision{
			mode: staticFileProcessingExecutionModeFullScan,
		}
	}
	return staticFileProcessingExecutionDecision{
		mode:             staticFileProcessingExecutionModeChangedPaths,
		changedFilePaths: append([]string(nil), changedFilePaths...),
	}
}

func shouldWriteFrameworkPublicFileMapTSForBuildDecision(
	shouldProcessPublicStaticFiles bool,
	frameworkPublicFileMapOutDir string,
) bool {
	return shouldProcessPublicStaticFiles && frameworkPublicFileMapOutDir != ""
}

func deriveBuildPhaseExecutionDecision(
	buildDecision buildPhaseDecision,
	frameworkPublicFileMapOutDir string,
) buildPhaseExecutionDecision {
	return buildPhaseExecutionDecision{
		compileGo: buildDecision.compileGo,
		publicStaticProcessing: deriveStaticFileProcessingExecutionDecision(
			buildDecision.processPublicFiles,
			buildDecision.publicStaticChangedFilePaths,
		),
		privateStaticProcessing: deriveStaticFileProcessingExecutionDecision(
			buildDecision.processPrivateFiles,
			buildDecision.privateStaticChangedFilePaths,
		),
		buildCriticalCSS: buildDecision.buildCriticalCSS,
		buildNormalCSS:   buildDecision.buildNormalCSS,
		writeFrameworkPublicFileMap: shouldWriteFrameworkPublicFileMapTSForBuildDecision(
			buildDecision.processPublicFiles,
			frameworkPublicFileMapOutDir,
		),
	}
}

func shouldExecuteAnyFileProcessingForBuildDecision(
	executionDecision buildPhaseExecutionDecision,
) bool {
	return executionDecision.publicStaticProcessing.shouldProcess() ||
		executionDecision.privateStaticProcessing.shouldProcess() ||
		executionDecision.buildCriticalCSS ||
		executionDecision.buildNormalCSS
}

func (s *server) executeBuildPhase(work *workSet) {
	builder := s.getBuilder()
	if builder == nil {
		s.log.Error("Builder is nil during build phase")
		return
	}

	buildExecutionDecision := deriveBuildPhaseExecutionDecision(
		work.build,
		s.cfg.FrameworkPublicFileMapOutDir,
	)

	var buildPhaseGroup errgroup.Group

	if buildExecutionDecision.compileGo {
		buildPhaseGroup.Go(func() error {
			if err := builder.CompileGoOnly(true); err != nil {
				s.log.Error("Go compilation failed", "error", err)
				return err
			}
			return nil
		})
	}

	if shouldExecuteAnyFileProcessingForBuildDecision(buildExecutionDecision) {
		buildPhaseGroup.Go(func() error {
			if err := s.executePublicStaticProcessingForBuildPhase(
				builder,
				buildExecutionDecision,
			); err != nil {
				return err
			}

			var assetAndCSSBuildGroup errgroup.Group

			if buildExecutionDecision.privateStaticProcessing.shouldProcess() {
				assetAndCSSBuildGroup.Go(func() error {
					return s.executePrivateStaticProcessingForBuildPhase(
						builder,
						buildExecutionDecision.privateStaticProcessing,
					)
				})
			}

			if buildExecutionDecision.buildCriticalCSS {
				assetAndCSSBuildGroup.Go(func() error {
					if err := builder.BuildCriticalCSS(true); err != nil {
						s.log.Error("Critical CSS build failed", "error", err)
						return err
					}
					return nil
				})
			}

			if buildExecutionDecision.buildNormalCSS {
				assetAndCSSBuildGroup.Go(func() error {
					if err := builder.BuildNormalCSS(true); err != nil {
						s.log.Error("Normal CSS build failed", "error", err)
						return err
					}
					return nil
				})
			}

			return assetAndCSSBuildGroup.Wait()
		})
	}

	if err := buildPhaseGroup.Wait(); err != nil {
		s.log.Error("Build phase had errors", "error", err)
	}
}

func (s *server) executePublicStaticProcessingForBuildPhase(
	builder *Builder,
	buildExecutionDecision buildPhaseExecutionDecision,
) error {
	if !buildExecutionDecision.publicStaticProcessing.shouldProcess() {
		return nil
	}

	publicStaticProcessingError := executeStaticFileProcessingForBuildPhase(
		builder.ProcessPublicFilesOnly,
		builder.processPublicFilesOnlyForChangedPaths,
		buildExecutionDecision.publicStaticProcessing,
	)
	if publicStaticProcessingError != nil {
		s.log.Error("Public files processing failed", "error", publicStaticProcessingError)
		return publicStaticProcessingError
	}

	if buildExecutionDecision.writeFrameworkPublicFileMap {
		if err := builder.WritePublicFileMapTS(s.cfg.FrameworkPublicFileMapOutDir); err != nil {
			s.log.Error("Write public file map TS failed", "error", err)
			return err
		}
	}

	return nil
}

func (s *server) executePrivateStaticProcessingForBuildPhase(
	builder *Builder,
	privateStaticProcessingDecision staticFileProcessingExecutionDecision,
) error {
	privateStaticProcessingError := executeStaticFileProcessingForBuildPhase(
		builder.ProcessPrivateFilesOnly,
		builder.processPrivateFilesOnlyForChangedPaths,
		privateStaticProcessingDecision,
	)
	if privateStaticProcessingError != nil {
		s.log.Error("Private files processing failed", "error", privateStaticProcessingError)
		return privateStaticProcessingError
	}
	return nil
}

func executeStaticFileProcessingForBuildPhase(
	executeFullStaticProcessing func() error,
	executeChangedPathsStaticProcessing func([]string) error,
	staticProcessingDecision staticFileProcessingExecutionDecision,
) error {
	switch staticProcessingDecision.mode {
	case staticFileProcessingExecutionModeNone:
		return nil

	case staticFileProcessingExecutionModeChangedPaths:
		if executeChangedPathsStaticProcessing == nil {
			return nil
		}
		return executeChangedPathsStaticProcessing(staticProcessingDecision.changedFilePaths)

	case staticFileProcessingExecutionModeFullScan:
		if executeFullStaticProcessing == nil {
			return nil
		}
		return executeFullStaticProcessing()

	default:
		return nil
	}
}
