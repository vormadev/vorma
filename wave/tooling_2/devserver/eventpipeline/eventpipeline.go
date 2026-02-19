package eventpipeline

import (
	"encoding/base64"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/internal/waveshared"
	"github.com/vormadev/vorma/wave/tooling_2/broadcast"
	"github.com/vormadev/vorma/wave/tooling_2/builder"
	"github.com/vormadev/vorma/wave/tooling_2/watch"
	"github.com/vormadev/vorma/wave/tooling_2/watch/classification"
)

// -----------------------------------------------------------------------------
// Core event and work modeling.
// -----------------------------------------------------------------------------
type FileType int

const (
	FileTypeOther FileType = iota
	FileTypeGo
	FileTypeCriticalCSS
	FileTypeNormalCSS
	FileTypeCriticalAndNormalCSS
	FileTypePublicStatic
	FileTypePrivateStatic
)

type ClassifiedEvent struct {
	Event       fsnotify.Event
	FileType    FileType
	WatchedFile *wave.WatchedFile
	Ignored     bool
	ChmodOnly   bool
}

// EventWithHooks pairs a classified event with its sorted hooks.
type EventWithHooks struct {
	Classified         ClassifiedEvent
	Hooks              *wave.SortedHooks
	HookCtx            *wave.HookContext
	RunOnChangeOnly    bool
	NeedsHardReload    bool
	SkipDuplicateHooks bool
}

type BuildPhaseDecision struct {
	CompileGo                       bool
	BuildCriticalCSS                bool
	BuildNormalCSS                  bool
	ProcessPublicFiles              bool
	ProcessPrivateFiles             bool
	PublicStaticChangedFilePaths    []string
	PrivateStaticChangedFilePaths   []string
	PublicStaticChangedFilePathSet  map[string]struct{}
	PrivateStaticChangedFilePathSet map[string]struct{}
}

type RestartPhaseDecision struct {
	RestartApp bool
}

type BrowserPhaseAction int

const (
	BrowserPhaseActionNone BrowserPhaseAction = iota
	BrowserPhaseActionHotReloadCSS
	BrowserPhaseActionRevalidate
	BrowserPhaseActionHardReload
	BrowserPhaseActionInvalidateVite
)

type BrowserPhaseDecision struct {
	Action      BrowserPhaseAction
	WaitForApp  bool
	WaitForVite bool
	CycleVite   bool
}

type BrowserPhaseResolution struct {
	Action         BrowserPhaseAction
	ApplyWaitFlags bool
	WaitForApp     bool
	WaitForVite    bool
}

// WorkSet collects per-phase execution decisions for a watcher cycle.
type WorkSet struct {
	Build   BuildPhaseDecision
	Restart RestartPhaseDecision
	Browser BrowserPhaseDecision

	// User preference collected from watched files and applied during resolve.
	PreferRevalidate bool
}

type AppStopStrategy int

const (
	AppStopStrategyNone AppStopStrategy = iota
	AppStopStrategySingleEventHardReload
	AppStopStrategyBatchHardReload
)

type EventExecutionPlanningResult struct {
	EventsWithHooks []EventWithHooks
	ConfigChanged   bool
}

type WatcherEventFlowDecision struct {
	TriggerConfigRestart       bool
	BroadcastRebuildingOverlay bool
	BehavioralDecision         EventExecutionPlanBehavioralDecision
}

type WatcherEventExecutionInput struct {
	FlowDecision            WatcherEventFlowDecision
	EventsWithHooks         []EventWithHooks
	WatcherEventLogPayloads []WatcherEventLogPayload
}

type EventExecutionPlanBehavioralDecision struct {
	ShowRebuildingOverlay bool
	AppStopStrategy       AppStopStrategy
	RunImplicitBuild      bool
}

type WatcherEventLogPayload struct {
	Operation string
	FilePath  string
}

type RefreshActionApplicationResult struct {
	RestartRequested bool
	RecompileGo      bool
}

type RefreshActionWorkMutationDecision struct {
	RestartApp           bool
	CompileGo            bool
	RequestBrowserAction bool
	BrowserAction        BrowserPhaseAction
	WaitForApp           bool
	WaitForVite          bool
}

// RefreshActionReductionDecision captures stable ordering and early stop
// behavior while reducing refresh actions.
type RefreshActionReductionDecision struct {
	ActionsBeforeRestart     []wave.RefreshAction
	RestartActionEncountered bool
	RestartActionIndex       int
	ApplicationResult        RefreshActionApplicationResult
}

type ImplicitWorkDecision struct {
	CompileGo                    bool
	BuildCriticalCSS             bool
	BuildNormalCSS               bool
	ProcessPublicFiles           bool
	ProcessPrivateFiles          bool
	RestartApp                   bool
	PreferRevalidate             bool
	PublicStaticChangedFilePath  string
	PrivateStaticChangedFilePath string
}

// ReloadOpts carries reload payload and readiness behavior for browser updates.
type ReloadOpts struct {
	Payload   broadcast.Payload
	WaitApp   bool
	WaitVite  bool
	CycleVite bool
}

// -----------------------------------------------------------------------------
// Browser phase planning.
// -----------------------------------------------------------------------------
func PlanBrowserReloadForAction(
	action BrowserPhaseAction,
	browserDecision BrowserPhaseDecision,
) (ReloadOpts, bool) {
	switch action {
	case BrowserPhaseActionHardReload:
		return ReloadOpts{
			Payload:   broadcast.Payload{ChangeType: broadcast.ChangeTypeOther},
			WaitApp:   browserDecision.WaitForApp,
			WaitVite:  browserDecision.WaitForVite,
			CycleVite: browserDecision.CycleVite,
		}, true
	case BrowserPhaseActionRevalidate:
		return ReloadOpts{
			Payload:   broadcast.Payload{ChangeType: broadcast.ChangeTypeRevalidate},
			WaitApp:   browserDecision.WaitForApp,
			WaitVite:  browserDecision.WaitForVite,
			CycleVite: false,
		}, true
	default:
		return ReloadOpts{}, false
	}
}

func PlanInvalidateViteFallbackBrowserDecision(
	usingVite bool,
) BrowserPhaseDecision {
	return BrowserPhaseDecision{
		Action:      BrowserPhaseActionHardReload,
		WaitForApp:  true,
		WaitForVite: usingVite,
	}
}

func PlanHotReloadCSSPayloads(
	includeCriticalCSS bool,
	criticalCSS string,
	criticalCSSAvailable bool,
	includeNormalCSS bool,
	normalCSSURL string,
	normalCSSURLAvailable bool,
) []broadcast.Payload {
	payloads := make([]broadcast.Payload, 0, 2)
	if includeCriticalCSS && criticalCSSAvailable {
		payloads = append(payloads, broadcast.Payload{
			ChangeType:  broadcast.ChangeTypeCriticalCSS,
			CriticalCSS: base64.StdEncoding.EncodeToString([]byte(criticalCSS)),
		})
	}
	if includeNormalCSS && normalCSSURLAvailable {
		payloads = append(payloads, broadcast.Payload{
			ChangeType:   broadcast.ChangeTypeNormalCSS,
			NormalCSSURL: normalCSSURL,
		})
	}
	return payloads
}

type BrowserPhaseExecutionCategory int

const (
	BrowserPhaseExecutionCategoryNone BrowserPhaseExecutionCategory = iota
	BrowserPhaseExecutionCategoryReload
	BrowserPhaseExecutionCategoryHotReloadCSS
)

func ShouldAttemptViteInvalidateForBrowserDecision(
	browserDecision BrowserPhaseDecision,
	usingVite bool,
) bool {
	return browserDecision.Action == BrowserPhaseActionInvalidateVite && usingVite
}

func ResolveBrowserDecisionAfterInvalidateViteFallback(
	browserDecision BrowserPhaseDecision,
	usingVite bool,
) BrowserPhaseDecision {
	if browserDecision.Action != BrowserPhaseActionInvalidateVite {
		return browserDecision
	}

	fallbackDecision := PlanInvalidateViteFallbackBrowserDecision(usingVite)
	browserDecision.Action = fallbackDecision.Action
	browserDecision.WaitForApp = fallbackDecision.WaitForApp
	browserDecision.WaitForVite = fallbackDecision.WaitForVite
	return browserDecision
}

func DeriveBrowserPhaseExecutionCategory(
	action BrowserPhaseAction,
) BrowserPhaseExecutionCategory {
	switch action {
	case BrowserPhaseActionHardReload, BrowserPhaseActionRevalidate:
		return BrowserPhaseExecutionCategoryReload
	case BrowserPhaseActionHotReloadCSS:
		return BrowserPhaseExecutionCategoryHotReloadCSS
	default:
		return BrowserPhaseExecutionCategoryNone
	}
}

// -----------------------------------------------------------------------------
// Build/static phase planning.
// -----------------------------------------------------------------------------
type StaticFileProcessingExecutionMode int

const (
	StaticFileProcessingExecutionModeNone StaticFileProcessingExecutionMode = iota
	StaticFileProcessingExecutionModeFullScan
	StaticFileProcessingExecutionModeChangedPaths
)

type StaticFileProcessingExecutionDecision struct {
	Mode             StaticFileProcessingExecutionMode
	ChangedFilePaths []string
}

func (decision StaticFileProcessingExecutionDecision) ShouldProcess() bool {
	return decision.Mode != StaticFileProcessingExecutionModeNone
}

type BuildPhaseExecutionDecision struct {
	CompileGo                   bool
	PublicStaticProcessing      StaticFileProcessingExecutionDecision
	PrivateStaticProcessing     StaticFileProcessingExecutionDecision
	BuildCriticalCSS            bool
	BuildNormalCSS              bool
	WriteFrameworkPublicFileMap bool
}

func ShouldExecuteBuildPhaseForExecutionDecision(
	executionDecision BuildPhaseExecutionDecision,
) bool {
	return executionDecision.CompileGo ||
		ShouldExecuteAnyFileProcessingForBuildDecision(executionDecision)
}

func DeriveStaticFileProcessingExecutionDecision(
	shouldProcess bool,
	changedFilePaths []string,
) StaticFileProcessingExecutionDecision {
	if !shouldProcess {
		return StaticFileProcessingExecutionDecision{}
	}
	if len(changedFilePaths) == 0 {
		return StaticFileProcessingExecutionDecision{
			Mode: StaticFileProcessingExecutionModeFullScan,
		}
	}
	return StaticFileProcessingExecutionDecision{
		Mode:             StaticFileProcessingExecutionModeChangedPaths,
		ChangedFilePaths: append([]string(nil), changedFilePaths...),
	}
}

func ShouldWriteFrameworkPublicFileMapTSForBuildDecision(
	shouldProcessPublicStaticFiles bool,
	frameworkPublicFileMapOutDir string,
) bool {
	return shouldProcessPublicStaticFiles && frameworkPublicFileMapOutDir != ""
}

func DeriveBuildPhaseExecutionDecision(
	buildDecision BuildPhaseDecision,
	frameworkPublicFileMapOutDir string,
) BuildPhaseExecutionDecision {
	return BuildPhaseExecutionDecision{
		CompileGo: buildDecision.CompileGo,
		PublicStaticProcessing: DeriveStaticFileProcessingExecutionDecision(
			buildDecision.ProcessPublicFiles,
			buildDecision.PublicStaticChangedFilePaths,
		),
		PrivateStaticProcessing: DeriveStaticFileProcessingExecutionDecision(
			buildDecision.ProcessPrivateFiles,
			buildDecision.PrivateStaticChangedFilePaths,
		),
		BuildCriticalCSS: buildDecision.BuildCriticalCSS,
		BuildNormalCSS:   buildDecision.BuildNormalCSS,
		WriteFrameworkPublicFileMap: ShouldWriteFrameworkPublicFileMapTSForBuildDecision(
			buildDecision.ProcessPublicFiles,
			frameworkPublicFileMapOutDir,
		),
	}
}

func ShouldExecuteAnyFileProcessingForBuildDecision(
	executionDecision BuildPhaseExecutionDecision,
) bool {
	return executionDecision.PublicStaticProcessing.ShouldProcess() ||
		executionDecision.PrivateStaticProcessing.ShouldProcess() ||
		executionDecision.BuildCriticalCSS ||
		executionDecision.BuildNormalCSS
}

func ExecuteStaticFileProcessingForBuildPhase(
	executeFullStaticProcessing func() error,
	executeChangedPathsStaticProcessing func([]string) error,
	staticProcessingDecision StaticFileProcessingExecutionDecision,
) error {
	switch staticProcessingDecision.Mode {
	case StaticFileProcessingExecutionModeNone:
		return nil

	case StaticFileProcessingExecutionModeChangedPaths:
		if executeChangedPathsStaticProcessing == nil {
			return nil
		}
		return executeChangedPathsStaticProcessing(staticProcessingDecision.ChangedFilePaths)

	case StaticFileProcessingExecutionModeFullScan:
		if executeFullStaticProcessing == nil {
			return nil
		}
		return executeFullStaticProcessing()

	default:
		return nil
	}
}

// ClassifyWatcherEventsForProcessing applies pre-classification side effects,
// maps events into file categories, then drops ignored/chmod-only entries.

// -----------------------------------------------------------------------------
// Watcher event classification and filtering helpers.
// -----------------------------------------------------------------------------
func FilterClassifiedEventsForProcessingByPostClassificationDecision(
	classifiedEvents []ClassifiedEvent,
) []ClassifiedEvent {
	if len(classifiedEvents) == 0 {
		return nil
	}

	filteredClassifiedEvents := make([]ClassifiedEvent, 0, len(classifiedEvents))
	for _, classifiedEventForProcessing := range classifiedEvents {
		postClassificationDecision := classification.DerivePostClassificationDecision(
			classifiedEventForProcessing.Ignored,
			classifiedEventForProcessing.ChmodOnly,
		)
		if !postClassificationDecision.IncludeClassifiedEvent {
			continue
		}
		filteredClassifiedEvents = append(
			filteredClassifiedEvents,
			classifiedEventForProcessing,
		)
	}
	return filteredClassifiedEvents
}

// ClassifyEventWithWatcherAndBuilder maps a raw watcher event into file type,
// watch metadata, ignore status, and chmod-only state used by later phases.
func deriveInitialFileTypeForWatcherEvent(
	watcherEventPath string,
	watcher *watch.Watcher,
	builder *toolingbuilder.Builder,
) FileType {
	isCriticalCSSFile := builder.IsCriticalCSSFile(watcherEventPath)
	isNormalCSSFile := builder.IsNormalCSSFile(watcherEventPath)

	if isCriticalCSSFile && isNormalCSSFile {
		return FileTypeCriticalAndNormalCSS
	}
	if isCriticalCSSFile {
		return FileTypeCriticalCSS
	}
	if isNormalCSSFile {
		return FileTypeNormalCSS
	}
	if filepath.Ext(watcherEventPath) == ".go" {
		return FileTypeGo
	}
	if watcher.IsPublicStaticFile(watcherEventPath) {
		return FileTypePublicStatic
	}
	if watcher.IsPrivateStaticFile(watcherEventPath) {
		return FileTypePrivateStatic
	}
	return FileTypeOther
}

// DeriveInitialFileTypeForWatcherEvent resolves baseline file type before
// watched-file overrides are applied.
func DeriveInitialFileTypeForWatcherEvent(
	watcherEventPath string,
	watcher *watch.Watcher,
	builder *toolingbuilder.Builder,
) FileType {
	return deriveInitialFileTypeForWatcherEvent(
		watcherEventPath,
		watcher,
		builder,
	)
}

// deriveFileTypeWithWatchedFileOverrides applies per-watched-file overrides
// after extension/pattern-based initial classification.
func deriveFileTypeWithWatchedFileOverrides(
	initialFileType FileType,
	watchedFile *wave.WatchedFile,
) FileType {
	if initialFileType == FileTypeGo &&
		watchedFile != nil &&
		watchedFile.TreatAsNonGo {
		return FileTypeOther
	}
	return initialFileType
}

// DeriveFileTypeWithWatchedFileOverrides applies per-watched-file overrides
// after extension/pattern-based initial classification.
func DeriveFileTypeWithWatchedFileOverrides(
	initialFileType FileType,
	watchedFile *wave.WatchedFile,
) FileType {
	return deriveFileTypeWithWatchedFileOverrides(initialFileType, watchedFile)
}

// deriveWatcherEventIgnoredStatus resolves whether a classified event should be
// ignored before post-classification filtering.
func deriveWatcherEventIgnoredStatus(
	initialIgnoredStatus bool,
	resolvedFileType FileType,
	watchedFile *wave.WatchedFile,
) bool {
	if initialIgnoredStatus {
		return true
	}
	return resolvedFileType == FileTypeOther && watchedFile == nil
}

// DeriveWatcherEventIgnoredStatus resolves whether a classified event should be
// ignored before post-classification filtering.
func DeriveWatcherEventIgnoredStatus(
	initialIgnoredStatus bool,
	resolvedFileType FileType,
	watchedFile *wave.WatchedFile,
) bool {
	return deriveWatcherEventIgnoredStatus(
		initialIgnoredStatus,
		resolvedFileType,
		watchedFile,
	)
}

// -----------------------------------------------------------------------------
// Event-plan behavioral decisions.
// -----------------------------------------------------------------------------
func DeriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
	eventsWithHooks []EventWithHooks,
) EventExecutionPlanBehavioralDecision {
	return EventExecutionPlanBehavioralDecision{
		ShowRebuildingOverlay: ShouldShowRebuildingOverlayForEventsWithHooks(eventsWithHooks),
		AppStopStrategy:       ResolveAppStopStrategy(eventsWithHooks),
		RunImplicitBuild:      ShouldRunImplicitBuildForEvents(eventsWithHooks),
	}
}

func ShouldShowRebuildingOverlayForEventsWithHooks(
	eventsWithHooks []EventWithHooks,
) bool {
	if len(eventsWithHooks) == 0 {
		return false
	}

	classifiedEvents := make([]ClassifiedEvent, 0, len(eventsWithHooks))
	for _, eventWithHooksForOverlay := range eventsWithHooks {
		classifiedEvents = append(classifiedEvents, eventWithHooksForOverlay.Classified)
	}
	return ShouldShowRebuildingOverlay(classifiedEvents)
}

func BuildWatcherEventLogPayloadsForEventsWithHooks(
	eventsWithHooks []EventWithHooks,
) []WatcherEventLogPayload {
	if len(eventsWithHooks) == 0 {
		return nil
	}

	watcherEventLogPayloads := make(
		[]WatcherEventLogPayload,
		0,
		len(eventsWithHooks),
	)
	for _, eventWithHooksForLogging := range eventsWithHooks {
		watcherEventLogPayloads = append(
			watcherEventLogPayloads,
			WatcherEventLogPayload{
				Operation: eventWithHooksForLogging.Classified.Event.Op.String(),
				FilePath:  eventWithHooksForLogging.Classified.Event.Name,
			},
		)
	}
	return watcherEventLogPayloads
}

func ShouldShowRebuildingOverlay(
	classifiedEvents []ClassifiedEvent,
) bool {
	for _, classifiedEventForOverlay := range classifiedEvents {
		if classifiedEventForOverlay.FileType == FileTypeCriticalCSS ||
			classifiedEventForOverlay.FileType == FileTypeNormalCSS ||
			classifiedEventForOverlay.FileType == FileTypeCriticalAndNormalCSS {
			continue
		}

		if shouldSuppressRebuildingNotificationForClassifiedEvent(
			classifiedEventForOverlay,
		) {
			continue
		}

		return true
	}

	return false
}

func shouldSuppressRebuildingNotificationForClassifiedEvent(
	classifiedEventForOverlay ClassifiedEvent,
) bool {
	watchedFileForOverlay := classifiedEventForOverlay.WatchedFile
	if watchedFileForOverlay == nil {
		return false
	}

	if watchedFileForOverlay.SkipRebuildingNotification {
		return true
	}

	return watchedFileForOverlay.OnlyRunClientDefinedRevalidateFunc &&
		classifiedEventForOverlay.FileType != FileTypeGo &&
		!NeedsHardReload(watchedFileForOverlay)
}

func AnyEventNeedsHardReload(eventsWithHooks []EventWithHooks) bool {
	for _, eventWithHooksForCheck := range eventsWithHooks {
		if eventWithHooksForCheck.NeedsHardReload {
			return true
		}
	}
	return false
}

func ResolveAppStopStrategy(eventsWithHooks []EventWithHooks) AppStopStrategy {
	if len(eventsWithHooks) == 0 {
		return AppStopStrategyNone
	}

	if len(eventsWithHooks) == 1 {
		if eventsWithHooks[0].NeedsHardReload {
			return AppStopStrategySingleEventHardReload
		}
		return AppStopStrategyNone
	}

	if AnyEventNeedsHardReload(eventsWithHooks) {
		return AppStopStrategyBatchHardReload
	}

	return AppStopStrategyNone
}

func ShouldRunImplicitBuildForEvents(eventsWithHooks []EventWithHooks) bool {
	for _, eventWithHooksForCheck := range eventsWithHooks {
		if !eventWithHooksForCheck.RunOnChangeOnly {
			return true
		}
	}
	return false
}

// -----------------------------------------------------------------------------
// Execution input shaping and work-set mutation.
// -----------------------------------------------------------------------------
func BuildWatcherEventExecutionInputFromPlanningResult(
	executionPlanningResult EventExecutionPlanningResult,
) WatcherEventExecutionInput {
	flowDecision := DeriveWatcherEventFlowDecisionFromPlanningResult(
		executionPlanningResult,
	)
	executionInput := WatcherEventExecutionInput{
		FlowDecision: flowDecision,
	}
	if flowDecision.TriggerConfigRestart || len(executionPlanningResult.EventsWithHooks) == 0 {
		return executionInput
	}
	executionInput.EventsWithHooks = executionPlanningResult.EventsWithHooks
	executionInput.WatcherEventLogPayloads = BuildWatcherEventLogPayloadsForEventsWithHooks(
		executionInput.EventsWithHooks,
	)
	return executionInput
}

func DeriveWatcherEventFlowDecisionFromPlanningResult(
	executionPlanningResult EventExecutionPlanningResult,
) WatcherEventFlowDecision {
	if executionPlanningResult.ConfigChanged {
		return WatcherEventFlowDecision{
			TriggerConfigRestart: true,
		}
	}
	if len(executionPlanningResult.EventsWithHooks) == 0 {
		return WatcherEventFlowDecision{}
	}
	behavioralDecisionForExecutionPlan := DeriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
		executionPlanningResult.EventsWithHooks,
	)

	return WatcherEventFlowDecision{
		BroadcastRebuildingOverlay: behavioralDecisionForExecutionPlan.ShowRebuildingOverlay,
		BehavioralDecision:         behavioralDecisionForExecutionPlan,
	}
}

// resolve determines browser behavior based on build work and user preferences.
func (work *WorkSet) Resolve(usingVite bool) {
	if work.Build.CompileGo {
		work.Restart.RestartApp = true
	}
	work.DetermineBrowserBehavior(usingVite)
}

func (work *WorkSet) DetermineBrowserBehavior(usingVite bool) {
	browserPhaseResolutionForWork := DeriveBrowserPhaseResolutionForWorkSet(
		work.Build,
		work.Restart,
		work.PreferRevalidate,
		usingVite,
	)
	work.requestBrowserAction(browserPhaseResolutionForWork.Action)
	if browserPhaseResolutionForWork.ApplyWaitFlags {
		work.Browser.WaitForApp = browserPhaseResolutionForWork.WaitForApp
		work.Browser.WaitForVite = browserPhaseResolutionForWork.WaitForVite
	}
}

func (work *WorkSet) requestBrowserAction(action BrowserPhaseAction) {
	if action > work.Browser.Action {
		work.Browser.Action = action
	}
}

func DeriveBrowserPhaseResolutionForWorkSet(
	buildDecision BuildPhaseDecision,
	restartDecision RestartPhaseDecision,
	preferRevalidate bool,
	usingVite bool,
) BrowserPhaseResolution {
	if shouldUseRestartBrowserResolution(restartDecision) {
		return BrowserPhaseResolution{
			Action:         BrowserPhaseActionHardReload,
			ApplyWaitFlags: true,
			WaitForApp:     true,
			WaitForVite:    usingVite,
		}
	}

	if shouldUseRevalidateBrowserResolution(preferRevalidate) {
		return BrowserPhaseResolution{
			Action:         BrowserPhaseActionRevalidate,
			ApplyWaitFlags: true,
			WaitForApp:     true,
			WaitForVite:    usingVite,
		}
	}

	if isCSSOnlyBuildWorkForBrowserPhase(buildDecision) {
		return BrowserPhaseResolution{
			Action: BrowserPhaseActionHotReloadCSS,
		}
	}

	if buildDecision.ProcessPublicFiles {
		return BrowserPhaseResolution{
			Action: BrowserPhaseActionInvalidateVite,
		}
	}

	if buildDecision.ProcessPrivateFiles || buildDecision.BuildCriticalCSS || buildDecision.BuildNormalCSS {
		return BrowserPhaseResolution{
			Action:         BrowserPhaseActionHardReload,
			ApplyWaitFlags: true,
			WaitForApp:     true,
			WaitForVite:    usingVite,
		}
	}

	return BrowserPhaseResolution{
		Action: BrowserPhaseActionNone,
	}
}

func shouldUseRestartBrowserResolution(
	restartDecision RestartPhaseDecision,
) bool {
	return restartDecision.RestartApp
}

func shouldUseRevalidateBrowserResolution(
	preferRevalidate bool,
) bool {

	return preferRevalidate
}

func isCSSOnlyBuildWorkForBrowserPhase(
	buildDecision BuildPhaseDecision,
) bool {
	cssWork := buildDecision.BuildCriticalCSS || buildDecision.BuildNormalCSS
	return cssWork &&
		!buildDecision.ProcessPublicFiles &&
		!buildDecision.ProcessPrivateFiles
}

// addImplicitWork adds build work implied by a file type.
func (work *WorkSet) AddImplicitWork(classifiedEventForWork ClassifiedEvent) {
	implicitWorkDecisionForClassifiedEvent := DeriveImplicitWorkDecisionForClassifiedEvent(classifiedEventForWork)
	work.ApplyImplicitWorkDecision(implicitWorkDecisionForClassifiedEvent)
}

func DeriveImplicitWorkDecisionForClassifiedEvent(
	classifiedEventForWork ClassifiedEvent,
) ImplicitWorkDecision {
	watchedFileForWork := classifiedEventForWork.WatchedFile
	if watchedFileForWork != nil && watchedFileForWork.RunOnChangeOnly {
		return ImplicitWorkDecision{}
	}

	decision := ImplicitWorkDecision{}
	if watchedFileForWork != nil && watchedFileForWork.OnlyRunClientDefinedRevalidateFunc {
		decision.PreferRevalidate = true
	}

	switch classifiedEventForWork.FileType {
	case FileTypeGo:
		decision.CompileGo = true
		decision.RestartApp = true

	case FileTypeCriticalCSS:
		decision.BuildCriticalCSS = true
		decision.RestartApp = watchedFileForWork != nil && NeedsHardReload(watchedFileForWork)

	case FileTypeNormalCSS:
		decision.BuildNormalCSS = true
		decision.RestartApp = watchedFileForWork != nil && NeedsHardReload(watchedFileForWork)

	case FileTypeCriticalAndNormalCSS:
		decision.BuildCriticalCSS = true
		decision.BuildNormalCSS = true
		decision.RestartApp = watchedFileForWork != nil && NeedsHardReload(watchedFileForWork)

	case FileTypePublicStatic:
		decision.ProcessPublicFiles = true
		decision.PublicStaticChangedFilePath = classifiedEventForWork.Event.Name

	case FileTypePrivateStatic:
		decision.ProcessPrivateFiles = true
		decision.PrivateStaticChangedFilePath = classifiedEventForWork.Event.Name

	case FileTypeOther:
		if watchedFileForWork != nil {
			decision.CompileGo = watchedFileForWork.RecompileGoBinary
			decision.RestartApp = watchedFileForWork.RestartApp || watchedFileForWork.RecompileGoBinary
		}
	}

	return decision
}

func (work *WorkSet) ApplyImplicitWorkDecision(
	decision ImplicitWorkDecision,
) {
	if decision.PreferRevalidate {
		work.PreferRevalidate = true
	}
	if decision.CompileGo {
		work.Build.CompileGo = true
	}
	if decision.BuildCriticalCSS {
		work.Build.BuildCriticalCSS = true
	}
	if decision.BuildNormalCSS {
		work.Build.BuildNormalCSS = true
	}
	if decision.ProcessPublicFiles {
		work.Build.ProcessPublicFiles = true
		work.Build.addPublicStaticChangedFilePath(decision.PublicStaticChangedFilePath)
	}
	if decision.ProcessPrivateFiles {
		work.Build.ProcessPrivateFiles = true
		work.Build.addPrivateStaticChangedFilePath(decision.PrivateStaticChangedFilePath)
	}
	if decision.RestartApp {
		work.Restart.RestartApp = true
	}
}
func normalizeChangedSourceFilePathForWorkSet(
	filePath string,
) string {
	return waveshared.Absolute(filePath)
}

func appendNormalizedFilePathIfMissing(
	existingFilePaths []string,
	existingFilePathSet map[string]struct{},
	filePath string,
) ([]string, map[string]struct{}) {
	normalizedFilePath := normalizeChangedSourceFilePathForWorkSet(filePath)
	if normalizedFilePath == "" {
		return existingFilePaths, existingFilePathSet
	}

	if existingFilePathSet == nil {
		existingFilePathSet = make(map[string]struct{}, len(existingFilePaths)+1)
		for _, existingFilePath := range existingFilePaths {
			existingFilePathSet[existingFilePath] = struct{}{}
		}
	}
	if _, alreadyExists := existingFilePathSet[normalizedFilePath]; alreadyExists {
		return existingFilePaths, existingFilePathSet
	}

	existingFilePaths = append(existingFilePaths, normalizedFilePath)
	existingFilePathSet[normalizedFilePath] = struct{}{}
	return existingFilePaths, existingFilePathSet
}

func (buildDecision *BuildPhaseDecision) addPublicStaticChangedFilePath(
	filePath string,
) {
	buildDecision.PublicStaticChangedFilePaths, buildDecision.PublicStaticChangedFilePathSet = appendNormalizedFilePathIfMissing(
		buildDecision.PublicStaticChangedFilePaths,
		buildDecision.PublicStaticChangedFilePathSet,
		filePath,
	)
}

func (buildDecision *BuildPhaseDecision) addPrivateStaticChangedFilePath(
	filePath string,
) {
	buildDecision.PrivateStaticChangedFilePaths, buildDecision.PrivateStaticChangedFilePathSet = appendNormalizedFilePathIfMissing(
		buildDecision.PrivateStaticChangedFilePaths,
		buildDecision.PrivateStaticChangedFilePathSet,
		filePath,
	)
}

// addFromRefreshAction merges a RefreshAction from a callback into the work set.
func (work *WorkSet) AddFromRefreshAction(action wave.RefreshAction) {
	workMutationDecision := DeriveRefreshActionWorkMutationDecision(action)
	work.ApplyRefreshActionWorkMutationDecision(workMutationDecision)
}

func DeriveRefreshActionWorkMutationDecision(
	action wave.RefreshAction,
) RefreshActionWorkMutationDecision {
	workMutationDecision := RefreshActionWorkMutationDecision{}
	if action.TriggerRestart {
		workMutationDecision.RestartApp = true
		workMutationDecision.CompileGo = action.RecompileGo
	}
	if action.ReloadBrowser {
		workMutationDecision.RequestBrowserAction = true
		workMutationDecision.BrowserAction = BrowserPhaseActionHardReload
	}
	workMutationDecision.WaitForApp = action.WaitForApp
	workMutationDecision.WaitForVite = action.WaitForVite
	return workMutationDecision
}

func (work *WorkSet) ApplyRefreshActionWorkMutationDecision(
	workMutationDecision RefreshActionWorkMutationDecision,
) {
	if workMutationDecision.RestartApp {
		work.Restart.RestartApp = true
	}
	if workMutationDecision.CompileGo {
		work.Build.CompileGo = true
	}
	if workMutationDecision.RequestBrowserAction {
		work.requestBrowserAction(workMutationDecision.BrowserAction)
	}
	if workMutationDecision.WaitForApp {
		work.Browser.WaitForApp = true
	}
	if workMutationDecision.WaitForVite {
		work.Browser.WaitForVite = true
	}
}

func (work *WorkSet) ApplyRefreshActions(
	actions []wave.RefreshAction,
) RefreshActionApplicationResult {
	reductionDecision := ReduceRefreshActionsInStableOrder(actions)
	for _, action := range reductionDecision.ActionsBeforeRestart {
		work.AddFromRefreshAction(action)
	}

	return reductionDecision.ApplicationResult
}

func ReduceRefreshActionsInStableOrder(
	actions []wave.RefreshAction,
) RefreshActionReductionDecision {
	reductionDecision := RefreshActionReductionDecision{
		ActionsBeforeRestart: make([]wave.RefreshAction, 0, len(actions)),
		RestartActionIndex:   -1,
	}

	for actionIndex, action := range actions {
		if action.TriggerRestart {
			reductionDecision.RestartActionEncountered = true
			reductionDecision.RestartActionIndex = actionIndex
			reductionDecision.ApplicationResult = RefreshActionApplicationResult{
				RestartRequested: true,
				RecompileGo:      action.RecompileGo,
			}
			return reductionDecision
		}
		reductionDecision.ActionsBeforeRestart = append(reductionDecision.ActionsBeforeRestart, action)
	}

	return reductionDecision
}
func NeedsHardReload(watchedFile *wave.WatchedFile) bool {
	if watchedFile == nil {
		return false
	}
	return watchedFile.RecompileGoBinary || watchedFile.RestartApp
}
