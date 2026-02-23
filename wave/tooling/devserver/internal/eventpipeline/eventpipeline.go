package eventpipeline

import (
	"encoding/base64"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/internal/wavecore"
	"github.com/vormadev/vorma/wave/tooling/builder"
	"github.com/vormadev/vorma/wave/tooling/internal/broadcast"
	"github.com/vormadev/vorma/wave/tooling/internal/watch"
	"github.com/vormadev/vorma/wave/tooling/internal/watch/classification"
)

// FileType is the semantic classification for a watcher path.
type FileType int

const (
	// FileTypeOther is an event not mapped to Wave build/reload semantics.
	FileTypeOther FileType = iota
	// FileTypeGo is a Go file that may require app rebuild/restart.
	FileTypeGo
	// FileTypeCriticalCSS is a critical CSS source.
	FileTypeCriticalCSS
	// FileTypeNormalCSS is a non-critical CSS source.
	FileTypeNormalCSS
	// FileTypeCriticalAndNormalCSS is a file used by both CSS pipelines.
	FileTypeCriticalAndNormalCSS
	// FileTypePublicStatic is a public static asset source.
	FileTypePublicStatic
	// FileTypePrivateStatic is a private static asset source.
	FileTypePrivateStatic
)

// ClassifiedEvent is one semantically classified watcher event.
type ClassifiedEvent struct {
	Event       fsnotify.Event
	FileType    FileType
	WatchedFile *wave.WatchedFile
	Ignored     bool
	ChmodOnly   bool
}

// EventWithHooks pairs classified event with sorted hook metadata.
type EventWithHooks struct {
	Classified         ClassifiedEvent
	Hooks              *wave.SortedHooks
	HookCtx            *wave.HookContext
	RunOnChangeOnly    bool
	NeedsHardReload    bool
	SkipDuplicateHooks bool
}

// BuildPhaseDecision captures aggregated build-stage work for one watcher batch.
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

// RestartPhaseDecision captures restart behavior for current watcher batch.
type RestartPhaseDecision struct {
	RestartApp bool
}

// BrowserPhaseAction describes browser-side behavior after build/restart work.
type BrowserPhaseAction int

const (
	// BrowserPhaseActionNone means no browser action is needed.
	BrowserPhaseActionNone BrowserPhaseAction = iota
	// BrowserPhaseActionHotReloadCSS sends CSS-only update payloads.
	BrowserPhaseActionHotReloadCSS
	// BrowserPhaseActionRevalidate triggers framework revalidation callback.
	BrowserPhaseActionRevalidate
	// BrowserPhaseActionHardReload triggers full page reload.
	BrowserPhaseActionHardReload
	// BrowserPhaseActionInvalidateVite asks devserver to call Vite invalidate endpoint.
	BrowserPhaseActionInvalidateVite
)

// BrowserPhaseDecision captures browser behavior selected for one batch.
type BrowserPhaseDecision struct {
	Action      BrowserPhaseAction
	WaitForApp  bool
	WaitForVite bool
	CycleVite   bool
}

// BrowserPhaseResolution describes final browser decision after fallback handling.
type BrowserPhaseResolution struct {
	Action         BrowserPhaseAction
	ApplyWaitFlags bool
	WaitForApp     bool
	WaitForVite    bool
}

// WorkSet is the aggregate mutable execution plan for one event batch.
type WorkSet struct {
	Build   BuildPhaseDecision
	Restart RestartPhaseDecision
	Browser BrowserPhaseDecision

	FrameworkRuntimeReloadRequests []wave.FrameworkRuntimeReloadRequest

	PreferRevalidate bool
}

// AppStopStrategy controls if app process should be stopped before pipeline execution.
type AppStopStrategy int

const (
	// AppStopStrategyNone keeps app running during current event batch.
	AppStopStrategyNone AppStopStrategy = iota
	// AppStopStrategySingleEventHardReload stops app for first-event hard reload semantics.
	AppStopStrategySingleEventHardReload
	// AppStopStrategyBatchHardReload stops app for full-batch hard reload semantics.
	AppStopStrategyBatchHardReload
)

// EventExecutionPlanningResult is the output of event classification + hook planning.
type EventExecutionPlanningResult struct {
	EventsWithHooks []EventWithHooks
	ConfigChanged   bool
}

// EventExecutionPlanBehavioralDecision captures control-flow behavior for execution.
type EventExecutionPlanBehavioralDecision struct {
	ShowRebuildingOverlay bool
	AppStopStrategy       AppStopStrategy
	RunImplicitBuild      bool
}

// WatcherEventFlowDecision captures high-level flow decisions for one watcher batch.
type WatcherEventFlowDecision struct {
	TriggerConfigRestart       bool
	BroadcastRebuildingOverlay bool
	BehavioralDecision         EventExecutionPlanBehavioralDecision
}

// WatcherEventLogPayload is one watcher log payload line.
type WatcherEventLogPayload struct {
	Operation string
	FilePath  string
}

// WatcherEventExecutionInput carries watcher decisions into deterministic execution.
type WatcherEventExecutionInput struct {
	FlowDecision            WatcherEventFlowDecision
	EventsWithHooks         []EventWithHooks
	WatcherEventLogPayloads []WatcherEventLogPayload
}

// RefreshActionApplicationResult captures effects of applying refresh actions.
type RefreshActionApplicationResult struct {
	RestartRequested bool
	RecompileGo      bool
}

// RefreshActionWorkMutationDecision resolves workset mutations for one refresh action.
type RefreshActionWorkMutationDecision struct {
	RestartApp                    bool
	CompileGo                     bool
	RequestBrowserAction          bool
	BrowserAction                 BrowserPhaseAction
	WaitForApp                    bool
	WaitForVite                   bool
	FrameworkRuntimeReloadRequest *wave.FrameworkRuntimeReloadRequest
}

// RefreshActionReductionDecision captures staged reduction metadata.
type RefreshActionReductionDecision struct {
	ActionsBeforeRestart []wave.RefreshAction
	ApplicationResult    RefreshActionApplicationResult
}

// ImplicitWorkDecision captures workset mutation derived from classified file type.
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

// ReloadOpts carries payload and readiness flags for browser reload broadcast.
type ReloadOpts struct {
	Payload                        broadcast.Payload
	WaitApp                        bool
	WaitVite                       bool
	CycleVite                      bool
	FrameworkRuntimeReloadRequests []wave.FrameworkRuntimeReloadRequest
}

// BrowserPhaseExecutionCategory groups browser action kinds for execution branching.
type BrowserPhaseExecutionCategory int

const (
	// BrowserPhaseExecutionCategoryNone means no browser work.
	BrowserPhaseExecutionCategoryNone BrowserPhaseExecutionCategory = iota
	// BrowserPhaseExecutionCategoryReload means full/revalidate reload behavior.
	BrowserPhaseExecutionCategoryReload
	// BrowserPhaseExecutionCategoryHotReloadCSS means CSS hot update payload behavior.
	BrowserPhaseExecutionCategoryHotReloadCSS
)

// StaticFileProcessingExecutionMode describes static processing strategy.
type StaticFileProcessingExecutionMode int

const (
	// StaticFileProcessingExecutionModeNone means static processing is not required.
	StaticFileProcessingExecutionModeNone StaticFileProcessingExecutionMode = iota
	// StaticFileProcessingExecutionModeFullScan means full directory scan processing.
	StaticFileProcessingExecutionModeFullScan
	// StaticFileProcessingExecutionModeChangedPaths means process only changed paths.
	StaticFileProcessingExecutionModeChangedPaths
)

// StaticFileProcessingExecutionDecision captures static processing mode and paths.
type StaticFileProcessingExecutionDecision struct {
	Mode             StaticFileProcessingExecutionMode
	ChangedFilePaths []string
}

// ShouldProcess reports whether static processing should run.
func (decision StaticFileProcessingExecutionDecision) ShouldProcess() bool {
	return decision.Mode != StaticFileProcessingExecutionModeNone
}

// BuildPhaseExecutionDecision resolves concrete build-stage work execution.
type BuildPhaseExecutionDecision struct {
	CompileGo                   bool
	PublicStaticProcessing      StaticFileProcessingExecutionDecision
	PrivateStaticProcessing     StaticFileProcessingExecutionDecision
	BuildCriticalCSS            bool
	BuildNormalCSS              bool
	WriteFrameworkPublicFileMap bool
}

// PlanBrowserReloadForAction derives reload options for hard/revalidate actions.
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
			Payload: broadcast.Payload{
				ChangeType: broadcast.ChangeTypeRevalidate,
			},
			WaitApp:   browserDecision.WaitForApp,
			WaitVite:  browserDecision.WaitForVite,
			CycleVite: false,
		}, true
	default:
		return ReloadOpts{}, false
	}
}

// PlanInvalidateViteFallbackBrowserDecision derives fallback when invalidate fails.
func PlanInvalidateViteFallbackBrowserDecision(
	usingVite bool,
) BrowserPhaseDecision {
	return BrowserPhaseDecision{
		Action:      BrowserPhaseActionHardReload,
		WaitForApp:  true,
		WaitForVite: usingVite,
	}
}

// PlanHotReloadCSSPayloads builds browser payloads for CSS hot updates.
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

// ShouldAttemptViteInvalidateForBrowserDecision reports whether invalidate should be attempted.
func ShouldAttemptViteInvalidateForBrowserDecision(
	browserDecision BrowserPhaseDecision,
	usingVite bool,
) bool {
	return browserDecision.Action == BrowserPhaseActionInvalidateVite &&
		usingVite
}

// ResolveBrowserDecisionAfterInvalidateViteFallback rewrites browser decision after invalidate failure.
func ResolveBrowserDecisionAfterInvalidateViteFallback(
	browserDecision BrowserPhaseDecision,
	usingVite bool,
) BrowserPhaseDecision {
	if browserDecision.Action != BrowserPhaseActionInvalidateVite {
		return browserDecision
	}
	fallback := PlanInvalidateViteFallbackBrowserDecision(usingVite)
	browserDecision.Action = fallback.Action
	browserDecision.WaitForApp = fallback.WaitForApp
	browserDecision.WaitForVite = fallback.WaitForVite
	return browserDecision
}

// DeriveBrowserPhaseExecutionCategory maps browser action to execution category.
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

// ShouldExecuteBuildPhaseForExecutionDecision reports whether build phase should run.
func ShouldExecuteBuildPhaseForExecutionDecision(
	executionDecision BuildPhaseExecutionDecision,
) bool {
	return executionDecision.CompileGo ||
		ShouldExecuteAnyFileProcessingForBuildDecision(executionDecision)
}

// DeriveStaticFileProcessingExecutionDecision resolves static processing mode.
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

// ShouldWriteFrameworkPublicFileMapTSForBuildDecision resolves filemap TS write behavior.
func ShouldWriteFrameworkPublicFileMapTSForBuildDecision(
	shouldProcessPublicStaticFiles bool,
	frameworkPublicFileMapOutDir string,
) bool {
	return shouldProcessPublicStaticFiles &&
		strings.TrimSpace(frameworkPublicFileMapOutDir) != ""
}

// DeriveBuildPhaseExecutionDecision resolves concrete build stage decisions.
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

// ShouldExecuteAnyFileProcessingForBuildDecision reports whether non-go build work exists.
func ShouldExecuteAnyFileProcessingForBuildDecision(
	executionDecision BuildPhaseExecutionDecision,
) bool {
	return executionDecision.PublicStaticProcessing.ShouldProcess() ||
		executionDecision.PrivateStaticProcessing.ShouldProcess() ||
		executionDecision.BuildCriticalCSS ||
		executionDecision.BuildNormalCSS
}

// ExecuteStaticFileProcessingForBuildPhase executes static processing decision callbacks.
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
		return executeChangedPathsStaticProcessing(
			staticProcessingDecision.ChangedFilePaths,
		)
	case StaticFileProcessingExecutionModeFullScan:
		if executeFullStaticProcessing == nil {
			return nil
		}
		return executeFullStaticProcessing()
	default:
		return nil
	}
}

// FilterClassifiedEventsForProcessingByPostClassificationDecision removes ignored/chmod events.
func FilterClassifiedEventsForProcessingByPostClassificationDecision(
	classifiedEvents []ClassifiedEvent,
) []ClassifiedEvent {
	if len(classifiedEvents) == 0 {
		return nil
	}
	filteredEvents := make([]ClassifiedEvent, 0, len(classifiedEvents))
	for _, classifiedEvent := range classifiedEvents {
		decision := classification.DerivePostClassificationDecision(
			classifiedEvent.Ignored,
			classifiedEvent.ChmodOnly,
		)
		if !decision.IncludeClassifiedEvent {
			continue
		}
		filteredEvents = append(filteredEvents, classifiedEvent)
	}
	return filteredEvents
}

// DeriveInitialFileTypeForWatcherEvent classifies path by CSS/go/static membership.
func DeriveInitialFileTypeForWatcherEvent(
	watcherEventPath string,
	watcher *watch.Watcher,
	builder *builder.Builder,
) FileType {
	isCriticalCSSFile := false
	isNormalCSSFile := false
	if builder != nil {
		isCriticalCSSFile = builder.IsCriticalCSSFile(watcherEventPath)
		isNormalCSSFile = builder.IsNormalCSSFile(watcherEventPath)
	}

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
	if watcher != nil && watcher.IsPublicStaticFile(watcherEventPath) {
		return FileTypePublicStatic
	}
	if watcher != nil && watcher.IsPrivateStaticFile(watcherEventPath) {
		return FileTypePrivateStatic
	}
	return FileTypeOther
}

// DeriveFileTypeWithWatchedFileOverrides applies watched-file semantic overrides.
func DeriveFileTypeWithWatchedFileOverrides(
	initialFileType FileType,
	watchedFile *wave.WatchedFile,
) FileType {
	if initialFileType == FileTypeGo && watchedFile != nil &&
		watchedFile.TreatAsNonGo {
		return FileTypeOther
	}
	return initialFileType
}

// DeriveWatcherEventIgnoredStatus resolves ignored state after semantic classification.
func DeriveWatcherEventIgnoredStatus(
	initialIgnoredStatus bool,
	resolvedFileType FileType,
	watchedFile *wave.WatchedFile,
) bool {
	if initialIgnoredStatus {
		return true
	}
	return resolvedFileType == FileTypeOther && watchedFile == nil
}

// DeriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks derives execution behavior.
func DeriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
	eventsWithHooks []EventWithHooks,
) EventExecutionPlanBehavioralDecision {
	return EventExecutionPlanBehavioralDecision{
		ShowRebuildingOverlay: ShouldShowRebuildingOverlayForEventsWithHooks(
			eventsWithHooks,
		),
		AppStopStrategy:  ResolveAppStopStrategy(eventsWithHooks),
		RunImplicitBuild: ShouldRunImplicitBuildForEvents(eventsWithHooks),
	}
}

// ShouldShowRebuildingOverlayForEventsWithHooks reports whether rebuilding overlay should display.
func ShouldShowRebuildingOverlayForEventsWithHooks(
	eventsWithHooks []EventWithHooks,
) bool {
	for _, eventWithHooks := range eventsWithHooks {
		if shouldClassifiedEventTriggerRebuildingOverlay(
			eventWithHooks.Classified,
		) {
			return true
		}
	}
	return false
}

// BuildWatcherEventLogPayloadsForEventsWithHooks builds watcher log payloads.
func BuildWatcherEventLogPayloadsForEventsWithHooks(
	eventsWithHooks []EventWithHooks,
) []WatcherEventLogPayload {
	if len(eventsWithHooks) == 0 {
		return nil
	}
	payloads := make([]WatcherEventLogPayload, 0, len(eventsWithHooks))
	for _, eventWithHooks := range eventsWithHooks {
		payloads = append(payloads, WatcherEventLogPayload{
			Operation: eventOperationString(eventWithHooks.Classified.Event.Op),
			FilePath:  eventWithHooks.Classified.Event.Name,
		})
	}
	return payloads
}

// ShouldShowRebuildingOverlay resolves overlay behavior from classified event list.
func ShouldShowRebuildingOverlay(classifiedEvents []ClassifiedEvent) bool {
	for _, classifiedEvent := range classifiedEvents {
		if shouldClassifiedEventTriggerRebuildingOverlay(classifiedEvent) {
			return true
		}
	}
	return false
}

func shouldClassifiedEventTriggerRebuildingOverlay(
	classifiedEvent ClassifiedEvent,
) bool {
	switch classifiedEvent.FileType {
	case FileTypeCriticalCSS, FileTypeNormalCSS, FileTypeCriticalAndNormalCSS:
		return false
	}

	if classifiedEvent.WatchedFile == nil {
		return true
	}
	if classifiedEvent.WatchedFile.SkipRebuildingNotification {
		return false
	}
	if classifiedEvent.WatchedFile.OnlyRunClientDefinedRevalidateFunc &&
		classifiedEvent.FileType != FileTypeGo {
		return false
	}
	return true
}

// AnyEventNeedsHardReload reports whether any event requires hard browser reload.
func AnyEventNeedsHardReload(eventsWithHooks []EventWithHooks) bool {
	for _, eventWithHooks := range eventsWithHooks {
		if eventWithHooks.NeedsHardReload {
			return true
		}
	}
	return false
}

// ResolveAppStopStrategy determines whether app should be stopped before execution.
func ResolveAppStopStrategy(eventsWithHooks []EventWithHooks) AppStopStrategy {
	if len(eventsWithHooks) == 0 {
		return AppStopStrategyNone
	}
	if len(eventsWithHooks) == 1 && eventsWithHooks[0].NeedsHardReload {
		return AppStopStrategySingleEventHardReload
	}
	if AnyEventNeedsHardReload(eventsWithHooks) {
		return AppStopStrategyBatchHardReload
	}
	return AppStopStrategyNone
}

// ShouldRunImplicitBuildForEvents resolves implicit build execution flag.
func ShouldRunImplicitBuildForEvents(eventsWithHooks []EventWithHooks) bool {
	for _, eventWithHooks := range eventsWithHooks {
		if !eventWithHooks.RunOnChangeOnly {
			return true
		}
	}
	return false
}

// BuildWatcherEventExecutionInputFromPlanningResult builds pipeline input from planning result.
func BuildWatcherEventExecutionInputFromPlanningResult(
	executionPlanningResult EventExecutionPlanningResult,
) WatcherEventExecutionInput {
	flowDecision := DeriveWatcherEventFlowDecisionFromPlanningResult(
		executionPlanningResult,
	)
	if flowDecision.TriggerConfigRestart {
		return WatcherEventExecutionInput{FlowDecision: flowDecision}
	}
	if len(executionPlanningResult.EventsWithHooks) == 0 {
		return WatcherEventExecutionInput{FlowDecision: flowDecision}
	}
	return WatcherEventExecutionInput{
		FlowDecision:    flowDecision,
		EventsWithHooks: executionPlanningResult.EventsWithHooks,
		WatcherEventLogPayloads: BuildWatcherEventLogPayloadsForEventsWithHooks(
			executionPlanningResult.EventsWithHooks,
		),
	}
}

// DeriveWatcherEventFlowDecisionFromPlanningResult derives top-level watcher flow decisions.
func DeriveWatcherEventFlowDecisionFromPlanningResult(
	executionPlanningResult EventExecutionPlanningResult,
) WatcherEventFlowDecision {
	if executionPlanningResult.ConfigChanged {
		return WatcherEventFlowDecision{TriggerConfigRestart: true}
	}

	behavioralDecision := DeriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
		executionPlanningResult.EventsWithHooks,
	)
	return WatcherEventFlowDecision{
		TriggerConfigRestart:       false,
		BroadcastRebuildingOverlay: behavioralDecision.ShowRebuildingOverlay,
		BehavioralDecision:         behavioralDecision,
	}
}

// DetermineBrowserBehavior derives browser behavior from aggregated work and preferences.
func (work *WorkSet) DetermineBrowserBehavior(usingVite bool) {
	if work == nil {
		return
	}

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

// DeriveBrowserPhaseResolutionForWorkSet resolves browser action and wait flags.
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

	if shouldUseRevalidateBrowserResolution(
		preferRevalidate,
		buildDecision,
	) {
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

	if buildDecision.ProcessPrivateFiles ||
		buildDecision.BuildCriticalCSS ||
		buildDecision.BuildNormalCSS {
		return BrowserPhaseResolution{
			Action:         BrowserPhaseActionHardReload,
			ApplyWaitFlags: true,
			WaitForApp:     true,
			WaitForVite:    usingVite,
		}
	}

	if buildDecision.ProcessPublicFiles {
		return BrowserPhaseResolution{
			Action: BrowserPhaseActionInvalidateVite,
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
	buildDecision BuildPhaseDecision,
) bool {
	return preferRevalidate && !buildDecision.ProcessPublicFiles
}

func isCSSOnlyBuildWorkForBrowserPhase(
	buildDecision BuildPhaseDecision,
) bool {
	cssWork := buildDecision.BuildCriticalCSS || buildDecision.BuildNormalCSS
	return cssWork &&
		!buildDecision.ProcessPublicFiles &&
		!buildDecision.ProcessPrivateFiles
}

// DeriveImplicitWorkDecisionForClassifiedEvent derives default work from file type.
func DeriveImplicitWorkDecisionForClassifiedEvent(
	classifiedEvent ClassifiedEvent,
) ImplicitWorkDecision {
	watchedFile := classifiedEvent.WatchedFile
	if watchedFile != nil && watchedFile.RunOnChangeOnly {
		return ImplicitWorkDecision{}
	}

	decision := ImplicitWorkDecision{}
	if watchedFile != nil && watchedFile.OnlyRunClientDefinedRevalidateFunc {
		decision.PreferRevalidate = true
	}

	switch classifiedEvent.FileType {
	case FileTypeGo:
		decision.CompileGo = true
		decision.RestartApp = true

	case FileTypeCriticalCSS:
		decision.BuildCriticalCSS = true
		decision.RestartApp = watchedFile != nil && NeedsHardReload(watchedFile)

	case FileTypeNormalCSS:
		decision.BuildNormalCSS = true
		decision.RestartApp = watchedFile != nil && NeedsHardReload(watchedFile)

	case FileTypeCriticalAndNormalCSS:
		decision.BuildCriticalCSS = true
		decision.BuildNormalCSS = true
		decision.RestartApp = watchedFile != nil && NeedsHardReload(watchedFile)

	case FileTypePublicStatic:
		decision.ProcessPublicFiles = true
		decision.PublicStaticChangedFilePath = classifiedEvent.Event.Name

	case FileTypePrivateStatic:
		decision.ProcessPrivateFiles = true
		decision.PrivateStaticChangedFilePath = classifiedEvent.Event.Name

	case FileTypeOther:
		if watchedFile != nil {
			decision.CompileGo = watchedFile.RecompileGoBinary
			decision.RestartApp = watchedFile.RestartApp ||
				watchedFile.RecompileGoBinary
		}
	}

	return decision
}

// ApplyImplicitWorkDecision applies one implicit decision into workset.
func (work *WorkSet) ApplyImplicitWorkDecision(
	implicitWorkDecision ImplicitWorkDecision,
) {
	if work == nil {
		return
	}

	if implicitWorkDecision.PreferRevalidate {
		work.PreferRevalidate = true
	}
	if implicitWorkDecision.CompileGo {
		work.Build.CompileGo = true
	}
	if implicitWorkDecision.BuildCriticalCSS {
		work.Build.BuildCriticalCSS = true
	}
	if implicitWorkDecision.BuildNormalCSS {
		work.Build.BuildNormalCSS = true
	}
	if implicitWorkDecision.ProcessPublicFiles {
		work.Build.ProcessPublicFiles = true
		work.Build.addPublicStaticChangedFilePath(
			implicitWorkDecision.PublicStaticChangedFilePath,
		)
	}
	if implicitWorkDecision.ProcessPrivateFiles {
		work.Build.ProcessPrivateFiles = true
		work.Build.addPrivateStaticChangedFilePath(
			implicitWorkDecision.PrivateStaticChangedFilePath,
		)
	}
	if implicitWorkDecision.RestartApp {
		work.Restart.RestartApp = true
	}
}

func normalizeChangedSourceFilePathForWorkSet(
	filePath string,
) string {
	return wavecore.Absolute(filePath)
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
		existingFilePathSet = make(
			map[string]struct{},
			len(existingFilePaths)+1,
		)
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
	buildDecision.PublicStaticChangedFilePaths,
		buildDecision.PublicStaticChangedFilePathSet = appendNormalizedFilePathIfMissing(
		buildDecision.PublicStaticChangedFilePaths,
		buildDecision.PublicStaticChangedFilePathSet,
		filePath,
	)
}

func (buildDecision *BuildPhaseDecision) addPrivateStaticChangedFilePath(
	filePath string,
) {
	buildDecision.PrivateStaticChangedFilePaths,
		buildDecision.PrivateStaticChangedFilePathSet = appendNormalizedFilePathIfMissing(
		buildDecision.PrivateStaticChangedFilePaths,
		buildDecision.PrivateStaticChangedFilePathSet,
		filePath,
	)
}

// AddImplicitWork derives and applies implicit work for classified event.
func (work *WorkSet) AddImplicitWork(classifiedEvent ClassifiedEvent) {
	if work == nil {
		return
	}
	implicitWorkDecision := DeriveImplicitWorkDecisionForClassifiedEvent(
		classifiedEvent,
	)
	work.ApplyImplicitWorkDecision(implicitWorkDecision)
}

// AddFromRefreshAction applies hook refresh action semantics into workset.
func (work *WorkSet) AddFromRefreshAction(
	refreshAction wave.RefreshAction,
) RefreshActionApplicationResult {
	if work == nil {
		return RefreshActionApplicationResult{}
	}
	refreshActionWorkMutationDecision := DeriveRefreshActionWorkMutationDecision(
		refreshAction,
	)
	return work.ApplyRefreshActionWorkMutationDecision(
		refreshActionWorkMutationDecision,
	)
}

// Resolve finalizes browser decision from aggregated work.
func (work *WorkSet) Resolve(usingVite bool) {
	if work == nil {
		return
	}

	if work.Build.CompileGo {
		work.Restart.RestartApp = true
	}
	work.DetermineBrowserBehavior(usingVite)
}

// NeedsHardReload reports whether watched file demands hard reload behavior.
func NeedsHardReload(watchedFile *wave.WatchedFile) bool {
	if watchedFile == nil {
		return false
	}
	return watchedFile.RecompileGoBinary || watchedFile.RestartApp
}

// DeriveRefreshActionWorkMutationDecision resolves workset mutation from refresh action.
func DeriveRefreshActionWorkMutationDecision(
	refreshAction wave.RefreshAction,
) RefreshActionWorkMutationDecision {
	decision := RefreshActionWorkMutationDecision{}

	if refreshAction.TriggerRestart {
		decision.RestartApp = true
		decision.CompileGo = refreshAction.RecompileGo
	}

	if refreshAction.ReloadBrowser {
		decision.RequestBrowserAction = true
		decision.BrowserAction = BrowserPhaseActionHardReload
	}
	decision.WaitForApp = refreshAction.WaitForApp
	decision.WaitForVite = refreshAction.WaitForVite
	decision.FrameworkRuntimeReloadRequest = refreshAction.FrameworkRuntimeReloadRequest

	return decision
}

// ApplyRefreshActionWorkMutationDecision applies refresh-action mutation decision.
func (work *WorkSet) ApplyRefreshActionWorkMutationDecision(
	mutationDecision RefreshActionWorkMutationDecision,
) RefreshActionApplicationResult {
	if work == nil {
		return RefreshActionApplicationResult{}
	}

	applicationResult := RefreshActionApplicationResult{}

	if mutationDecision.RestartApp {
		work.Restart.RestartApp = true
		work.Build.CompileGo = work.Build.CompileGo ||
			mutationDecision.CompileGo
		applicationResult.RestartRequested = true
		applicationResult.RecompileGo = mutationDecision.CompileGo
	}

	if mutationDecision.RequestBrowserAction {
		work.requestBrowserAction(mutationDecision.BrowserAction)
	}
	if mutationDecision.WaitForApp {
		work.Browser.WaitForApp = true
	}
	if mutationDecision.WaitForVite {
		work.Browser.WaitForVite = true
	}
	if mutationDecision.FrameworkRuntimeReloadRequest != nil {
		work.FrameworkRuntimeReloadRequests = appendFrameworkRuntimeReloadRequestIfMissing(
			work.FrameworkRuntimeReloadRequests,
			*mutationDecision.FrameworkRuntimeReloadRequest,
		)
	}

	return applicationResult
}

func appendFrameworkRuntimeReloadRequestIfMissing(
	existingRequests []wave.FrameworkRuntimeReloadRequest,
	request wave.FrameworkRuntimeReloadRequest,
) []wave.FrameworkRuntimeReloadRequest {
	normalizedRequest, ok := normalizeFrameworkRuntimeReloadRequest(request)
	if !ok {
		return existingRequests
	}

	for _, existingRequest := range existingRequests {
		if existingRequest == normalizedRequest {
			return existingRequests
		}
	}

	return append(existingRequests, normalizedRequest)
}

func normalizeFrameworkRuntimeReloadRequest(
	request wave.FrameworkRuntimeReloadRequest,
) (wave.FrameworkRuntimeReloadRequest, bool) {
	normalizedEndpointPath := strings.TrimSpace(request.EndpointPath)
	if normalizedEndpointPath == "" {
		return wave.FrameworkRuntimeReloadRequest{}, false
	}
	if !strings.HasPrefix(normalizedEndpointPath, "/") {
		normalizedEndpointPath = "/" + normalizedEndpointPath
	}

	return wave.FrameworkRuntimeReloadRequest{
		EndpointPath:    normalizedEndpointPath,
		ReloadAttemptID: strings.TrimSpace(request.ReloadAttemptID),
		ExpectedBuildID: strings.TrimSpace(request.ExpectedBuildID),
		ReloadTrigger:   strings.TrimSpace(request.ReloadTrigger),
	}, true
}

// ApplyRefreshActions applies refresh actions in order until restart short-circuit.
func (work *WorkSet) ApplyRefreshActions(
	refreshActions []wave.RefreshAction,
) RefreshActionApplicationResult {
	if work == nil || len(refreshActions) == 0 {
		return RefreshActionApplicationResult{}
	}

	reductionDecision := ReduceRefreshActionsInStableOrder(refreshActions)
	for _, refreshAction := range reductionDecision.ActionsBeforeRestart {
		work.AddFromRefreshAction(refreshAction)
	}

	return reductionDecision.ApplicationResult
}

// ReduceRefreshActionsInStableOrder reduces actions while preserving pre-restart order.
func ReduceRefreshActionsInStableOrder(
	refreshActions []wave.RefreshAction,
) RefreshActionReductionDecision {
	reductionDecision := RefreshActionReductionDecision{
		ActionsBeforeRestart: make(
			[]wave.RefreshAction,
			0,
			len(refreshActions),
		),
	}

	restartRequested := false
	recompileGo := false
	for _, refreshAction := range refreshActions {
		if refreshAction.TriggerRestart {
			restartRequested = true
			recompileGo = recompileGo || refreshAction.RecompileGo
			continue
		}
		if restartRequested {
			continue
		}

		reductionDecision.ActionsBeforeRestart = append(
			reductionDecision.ActionsBeforeRestart,
			refreshAction,
		)
	}

	reductionDecision.ApplicationResult = RefreshActionApplicationResult{
		RestartRequested: restartRequested,
		RecompileGo:      recompileGo,
	}
	return reductionDecision
}

// eventOperationString derives concise watcher operation label.
func eventOperationString(operation fsnotify.Op) string {
	if operation.Has(fsnotify.Create) {
		return "CREATE"
	}
	if operation.Has(fsnotify.Write) {
		return "WRITE"
	}
	if operation.Has(fsnotify.Remove) {
		return "REMOVE"
	}
	if operation.Has(fsnotify.Rename) {
		return "RENAME"
	}
	if operation.Has(fsnotify.Chmod) {
		return "CHMOD"
	}
	return "UNKNOWN"
}

// NormalizeChangedPath canonicalizes changed file paths for deterministic sets.
func NormalizeChangedPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	normalizedPath := filepath.Clean(path)
	return strings.ReplaceAll(normalizedPath, "\\", "/")
}

// HasPathAlias checks path equivalence using Wave shared alias semantics.
func HasPathAlias(path string, aliases []string) bool {
	for _, alias := range aliases {
		if wavecore.PathsReferToSameLocation(path, alias) {
			return true
		}
	}
	return false
}
