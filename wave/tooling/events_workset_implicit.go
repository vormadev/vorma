package tooling

// addImplicitWork adds build work implied by a file type.
func (work *workSet) addImplicitWork(classifiedEventForWork classifiedEvent) {
	implicitWorkDecisionForClassifiedEvent := deriveImplicitWorkDecisionForClassifiedEvent(classifiedEventForWork)
	work.applyImplicitWorkDecision(implicitWorkDecisionForClassifiedEvent)
}

func deriveImplicitWorkDecisionForClassifiedEvent(
	classifiedEventForWork classifiedEvent,
) implicitWorkDecision {
	watchedFileForWork := classifiedEventForWork.watchedFile
	if watchedFileForWork != nil && watchedFileForWork.RunOnChangeOnly {
		return implicitWorkDecision{}
	}

	decision := implicitWorkDecision{}
	if watchedFileForWork != nil && watchedFileForWork.OnlyRunClientDefinedRevalidateFunc {
		decision.preferRevalidate = true
	}

	switch classifiedEventForWork.fileType {
	case fileTypeGo:
		decision.compileGo = true
		decision.restartApp = true

	case fileTypeCriticalCSS:
		decision.buildCriticalCSS = true
		decision.restartApp = watchedFileForWork != nil && needsHardReload(watchedFileForWork)

	case fileTypeNormalCSS:
		decision.buildNormalCSS = true
		decision.restartApp = watchedFileForWork != nil && needsHardReload(watchedFileForWork)

	case fileTypeCriticalAndNormalCSS:
		decision.buildCriticalCSS = true
		decision.buildNormalCSS = true
		decision.restartApp = watchedFileForWork != nil && needsHardReload(watchedFileForWork)

	case fileTypePublicStatic:
		decision.processPublicFiles = true
		decision.publicStaticChangedFilePath = classifiedEventForWork.event.Name

	case fileTypePrivateStatic:
		decision.processPrivateFiles = true
		decision.privateStaticChangedFilePath = classifiedEventForWork.event.Name

	case fileTypeOther:
		if watchedFileForWork != nil {
			decision.compileGo = watchedFileForWork.RecompileGoBinary
			decision.restartApp = watchedFileForWork.RestartApp || watchedFileForWork.RecompileGoBinary
		}
	}

	return decision
}

func (work *workSet) applyImplicitWorkDecision(
	decision implicitWorkDecision,
) {
	if decision.preferRevalidate {
		work.preferRevalidate = true
	}
	if decision.compileGo {
		work.build.compileGo = true
	}
	if decision.buildCriticalCSS {
		work.build.buildCriticalCSS = true
	}
	if decision.buildNormalCSS {
		work.build.buildNormalCSS = true
	}
	if decision.processPublicFiles {
		work.build.processPublicFiles = true
		work.build.addPublicStaticChangedFilePath(decision.publicStaticChangedFilePath)
	}
	if decision.processPrivateFiles {
		work.build.processPrivateFiles = true
		work.build.addPrivateStaticChangedFilePath(decision.privateStaticChangedFilePath)
	}
	if decision.restartApp {
		work.restart.restartApp = true
	}
}
