// Package reloadwait isolates browser runtime orchestration for devserver.
//
// Devserver uses this package to keep refresh-server lifecycle, generation-
// based cancellation, readiness waiting, framework runtime reload hooks, and
// final browser payload broadcasts in one place.
package reloadwait

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/wavedev/devserver/internal/appsupervisor"
	"github.com/vormadev/vorma/wave/wavedev/devserver/internal/eventpipeline"
	"github.com/vormadev/vorma/wave/wavedev/internal/broadcast"
)

// CancelableGeneration tracks a monotonically increasing generation id and the
// active cancellation handle for that generation.
type CancelableGeneration struct {
	mutex      sync.Mutex
	generation uint64
	cancel     context.CancelFunc
}

// BeginNextGenerationAndClearCancel increments generation and returns any
// previously tracked cancellation handle.
func (tracker *CancelableGeneration) BeginNextGenerationAndClearCancel() (
	uint64,
	context.CancelFunc,
) {
	if tracker == nil {
		return 0, nil
	}
	tracker.mutex.Lock()
	defer tracker.mutex.Unlock()
	tracker.generation++
	currentGeneration := tracker.generation
	previousCancel := tracker.cancel
	tracker.cancel = nil
	return currentGeneration, previousCancel
}

// SetCancelForGeneration installs cancel only if generation is still current.
func (tracker *CancelableGeneration) SetCancelForGeneration(
	generation uint64,
	cancel context.CancelFunc,
) bool {
	if tracker == nil {
		return false
	}
	tracker.mutex.Lock()
	defer tracker.mutex.Unlock()
	if generation != tracker.generation {
		return false
	}
	tracker.cancel = cancel
	return true
}

// ClearCancelForGeneration clears cancel only if generation is still current.
func (tracker *CancelableGeneration) ClearCancelForGeneration(
	generation uint64,
) {
	if tracker == nil {
		return
	}
	tracker.mutex.Lock()
	defer tracker.mutex.Unlock()
	if generation == tracker.generation {
		tracker.cancel = nil
	}
}

// IsGenerationCurrent reports whether generation still matches current value.
func (tracker *CancelableGeneration) IsGenerationCurrent(
	generation uint64,
) bool {
	if tracker == nil {
		return false
	}
	tracker.mutex.Lock()
	defer tracker.mutex.Unlock()
	return generation == tracker.generation
}

// CancelAndAdvance advances generation and cancels the previously active handle.
func (tracker *CancelableGeneration) CancelAndAdvance() {
	if tracker == nil {
		return
	}
	_, previousCancel := tracker.BeginNextGenerationAndClearCancel()
	if previousCancel != nil {
		previousCancel()
	}
}

// ExecuteInvalidateAsyncOptions configures one asynchronous Vite invalidate
// attempt.
type ExecuteInvalidateAsyncOptions struct {
	BrowserDecision eventpipeline.BrowserPhaseDecision

	InvalidateGeneration uint64

	SetCancelForGeneration func(
		invalidateGeneration uint64,
		invalidateCancel context.CancelFunc,
	) bool
	ClearCancelForGeneration func(
		invalidateGeneration uint64,
	)
	IsGenerationCurrent func(
		invalidateGeneration uint64,
	) bool

	LaunchAsyncWork func(
		runAsyncWork func(context.Context),
	)

	CallViteFilemapInvalidateWithContext func(context.Context) error
	BroadcastReload                      func(eventpipeline.ReloadOpts)

	UsingVite bool
	Log       *slog.Logger
}

// ExecuteInvalidateAsync runs invalidate outside watcher critical path and
// falls back to hard reload when invalidate fails.
func ExecuteInvalidateAsync(options ExecuteInvalidateAsyncOptions) {
	launchAsyncWork := options.LaunchAsyncWork
	if launchAsyncWork == nil {
		launchAsyncWork = func(runAsyncWork func(context.Context)) {
			if runAsyncWork == nil {
				return
			}
			go runAsyncWork(context.Background())
		}
	}

	launchAsyncWork(
		func(cycleContext context.Context) {
			if cycleContext == nil {
				cycleContext = context.Background()
			}

			invalidateContext, cancelInvalidateContext := context.WithCancel(
				cycleContext,
			)
			if options.SetCancelForGeneration == nil ||
				!options.SetCancelForGeneration(
					options.InvalidateGeneration,
					cancelInvalidateContext,
				) {
				cancelInvalidateContext()
				return
			}
			defer cancelInvalidateContext()
			if options.ClearCancelForGeneration != nil {
				defer options.ClearCancelForGeneration(
					options.InvalidateGeneration,
				)
			}

			if options.CallViteFilemapInvalidateWithContext == nil {
				return
			}
			invalidateError := options.CallViteFilemapInvalidateWithContext(
				invalidateContext,
			)
			if invalidateError == nil {
				return
			}
			if invalidateContext.Err() != nil {
				return
			}
			if options.IsGenerationCurrent != nil &&
				!options.IsGenerationCurrent(options.InvalidateGeneration) {
				return
			}
			if options.Log != nil {
				options.Log.Warn(
					"vite invalidate endpoint failed; falling back to hard reload",
					"error",
					invalidateError,
				)
			}

			fallbackDecision := eventpipeline.ResolveBrowserDecisionAfterInvalidateViteFallback(
				options.BrowserDecision,
				options.UsingVite,
			)
			reloadOptions, hasReloadOptions := eventpipeline.PlanBrowserReloadForAction(
				fallbackDecision.Action,
				fallbackDecision,
			)
			if !hasReloadOptions || options.BroadcastReload == nil {
				return
			}
			options.BroadcastReload(reloadOptions)
		},
	)
}

// BroadcastReloadPayloadIfGenerationCurrentOptions configures conditional
// payload broadcast.
type BroadcastReloadPayloadIfGenerationCurrentOptions struct {
	ReloadBroadcastGeneration uint64
	Payload                   broadcast.Payload

	IsGenerationCurrent   func(uint64) bool
	CurrentRefreshManager func() *broadcast.Manager
}

// BroadcastReloadPayloadIfGenerationCurrent sends payload only if generation is
// still current before and after refresh-manager lookup.
func BroadcastReloadPayloadIfGenerationCurrent(
	options BroadcastReloadPayloadIfGenerationCurrentOptions,
) {
	if options.IsGenerationCurrent != nil &&
		!options.IsGenerationCurrent(options.ReloadBroadcastGeneration) {
		return
	}
	if options.CurrentRefreshManager == nil {
		return
	}
	refreshManager := options.CurrentRefreshManager()
	if refreshManager == nil {
		return
	}
	if options.IsGenerationCurrent != nil &&
		!options.IsGenerationCurrent(options.ReloadBroadcastGeneration) {
		return
	}
	refreshManager.Broadcast(options.Payload)
}

// BroadcastReloadAfterReadinessWithGenerationOptions configures deferred
// reload broadcast behavior.
type BroadcastReloadAfterReadinessWithGenerationOptions struct {
	ReadinessContext context.Context
	ReadinessCancel  context.CancelFunc

	ReloadBroadcastGeneration uint64
	ReloadOptions             eventpipeline.ReloadOpts

	ClearCancelForGeneration func(uint64)
	IsGenerationCurrent      func(uint64) bool

	WaitForReloadReadiness func(
		context.Context,
		eventpipeline.ReloadOpts,
	) bool
	ExecuteFrameworkRuntimeReloadRequestsWithContext func(
		context.Context,
		[]wave.FrameworkRuntimeReloadRequest,
	) error

	TriggerRestartNoGo func()

	ShouldBroadcastReloadPayloadAfterReadiness func(
		eventpipeline.ReloadOpts,
	) bool
	BroadcastReloadPayloadIfGenerationCurrent func(
		uint64,
		broadcast.Payload,
	)

	Log *slog.Logger
}

// BroadcastReloadAfterReadinessWithGeneration waits for readiness, performs
// framework runtime reload requests, and then broadcasts when still current.
func BroadcastReloadAfterReadinessWithGeneration(
	options BroadcastReloadAfterReadinessWithGenerationOptions,
) {
	if options.ReadinessCancel != nil {
		defer options.ReadinessCancel()
	}
	if options.ClearCancelForGeneration != nil {
		defer options.ClearCancelForGeneration(
			options.ReloadBroadcastGeneration,
		)
	}

	reloadOptionsForReadiness := options.ReloadOptions
	if len(reloadOptionsForReadiness.FrameworkRuntimeReloadRequests) > 0 {
		reloadOptionsForReadiness.WaitApp = true
	}

	if options.WaitForReloadReadiness != nil &&
		!options.WaitForReloadReadiness(
			options.ReadinessContext,
			reloadOptionsForReadiness,
		) {
		if options.ReadinessContext == nil ||
			options.ReadinessContext.Err() == nil {
			if options.Log != nil {
				options.Log.Warn(
					"reload readiness failed; skipping browser broadcast",
				)
			}
		}
		return
	}

	if options.ExecuteFrameworkRuntimeReloadRequestsWithContext != nil {
		frameworkRuntimeReloadError := options.ExecuteFrameworkRuntimeReloadRequestsWithContext(
			options.ReadinessContext,
			reloadOptionsForReadiness.FrameworkRuntimeReloadRequests,
		)
		if frameworkRuntimeReloadError != nil {
			if options.ReadinessContext == nil ||
				options.ReadinessContext.Err() == nil {
				if options.IsGenerationCurrent == nil ||
					options.IsGenerationCurrent(
						options.ReloadBroadcastGeneration,
					) {
					if options.Log != nil {
						options.Log.Warn(
							"framework runtime reload request failed; scheduling restart without go recompilation",
							"error",
							frameworkRuntimeReloadError,
						)
					}
					if options.TriggerRestartNoGo != nil {
						options.TriggerRestartNoGo()
					}
				}
			}
			return
		}
	}

	if options.IsGenerationCurrent != nil &&
		!options.IsGenerationCurrent(options.ReloadBroadcastGeneration) {
		return
	}
	if options.ShouldBroadcastReloadPayloadAfterReadiness != nil &&
		!options.ShouldBroadcastReloadPayloadAfterReadiness(
			reloadOptionsForReadiness,
		) {
		return
	}
	if options.BroadcastReloadPayloadIfGenerationCurrent != nil {
		options.BroadcastReloadPayloadIfGenerationCurrent(
			options.ReloadBroadcastGeneration,
			options.ReloadOptions.Payload,
		)
	}
}

// WaitForReloadReadinessOptions configures reload readiness policy execution.
type WaitForReloadReadinessOptions struct {
	ReadinessContext context.Context
	ReloadOptions    eventpipeline.ReloadOpts

	UsingVite bool

	IsViteRunning                           func() bool
	CycleViteAndWaitForReadinessWithContext func(context.Context) bool
	WaitForAppWithContext                   func(context.Context) bool
	WaitForViteWithContext                  func(context.Context) bool

	Log *slog.Logger
}

// WaitForReloadReadiness applies readiness policy for reload options.
func WaitForReloadReadiness(
	options WaitForReloadReadinessOptions,
) bool {
	if options.ReloadOptions.CycleVite {
		cycleViteRequestedAndApplicable := false
		if options.UsingVite && options.IsViteRunning != nil {
			cycleViteRequestedAndApplicable = options.IsViteRunning()
		}
		if cycleViteRequestedAndApplicable &&
			options.CycleViteAndWaitForReadinessWithContext != nil {
			if !options.CycleViteAndWaitForReadinessWithContext(
				options.ReadinessContext,
			) {
				if options.ReadinessContext != nil &&
					options.ReadinessContext.Err() != nil {
					return false
				}
				if options.Log != nil {
					options.Log.Warn(
						"cycle vite readiness failed; falling back to payload broadcast",
					)
				}
			}
		}
	}

	if options.ReloadOptions.WaitApp {
		if options.WaitForAppWithContext == nil ||
			!options.WaitForAppWithContext(options.ReadinessContext) {
			return false
		}
	}

	if options.ReloadOptions.WaitVite && options.IsViteRunning != nil &&
		options.IsViteRunning() {
		if options.WaitForViteWithContext == nil ||
			!options.WaitForViteWithContext(options.ReadinessContext) {
			return false
		}
	}
	return true
}

// ShouldBroadcastReloadPayloadAfterReadiness reports whether payload should be
// sent after readiness checks complete.
func ShouldBroadcastReloadPayloadAfterReadiness(
	reloadOptions eventpipeline.ReloadOpts,
	usingVite bool,
	viteRunning bool,
) bool {
	if !reloadOptions.CycleVite {
		return true
	}
	if !usingVite {
		return true
	}
	return !viteRunning
}

// ResolveViteReadyURL resolves the canonical Vite readiness probe URL.
func ResolveViteReadyURL(vitePort int) string {
	return appsupervisor.ResolveReadinessProbeURL(
		appsupervisor.LocalReadinessProbeHostIPv4,
		vitePort,
		"/@vite/client",
	)
}

// ResolveViteReadyURLs resolves all supported Vite readiness probe URLs.
func ResolveViteReadyURLs(vitePort int) []string {
	return []string{
		appsupervisor.ResolveReadinessProbeURL(
			appsupervisor.LocalReadinessProbeHostIPv4,
			vitePort,
			"/@vite/client",
		),
		appsupervisor.ResolveReadinessProbeURL(
			appsupervisor.LocalReadinessProbeHostLocalhost,
			vitePort,
			"/@vite/client",
		),
	}
}

// CallViteFilemapInvalidateWithContext calls the Vite invalidate endpoint with
// explicit cancellation context.
func CallViteFilemapInvalidateWithContext(
	invalidateContext context.Context,
	vitePort int,
) error {
	if invalidateContext == nil {
		invalidateContext = context.Background()
	}

	invalidateURL := "http://127.0.0.1:" + strconv.Itoa(vitePort) +
		"/__vorma_invalidate_filemap"
	request, requestCreateError := http.NewRequest(
		http.MethodPost,
		invalidateURL,
		nil,
	)
	if requestCreateError != nil {
		return requestCreateError
	}
	request = request.WithContext(invalidateContext)
	response, requestError := (&http.Client{}).Do(request)
	if requestError != nil {
		return requestError
	}
	defer response.Body.Close()

	if response.StatusCode >= 400 {
		return fmt.Errorf(
			"vite invalidate endpoint returned %d",
			response.StatusCode,
		)
	}
	return nil
}

// CallFrameworkRuntimeReloadEndpointWithContext posts to one framework runtime
// reload endpoint exposed by the running app process.
func CallFrameworkRuntimeReloadEndpointWithContext(
	reloadContext context.Context,
	appPort int,
	reloadRequest wave.FrameworkRuntimeReloadRequest,
) error {
	normalizedEndpointPath := strings.TrimSpace(reloadRequest.EndpointPath)
	if normalizedEndpointPath == "" {
		return errors.New("framework runtime reload endpoint path is required")
	}
	if !strings.HasPrefix(normalizedEndpointPath, "/") {
		normalizedEndpointPath = "/" + normalizedEndpointPath
	}

	if reloadContext == nil {
		reloadContext = context.Background()
	}
	reloadURL := appsupervisor.ResolveReadinessProbeURL(
		appsupervisor.LocalReadinessProbeHostIPv4,
		appPort,
		normalizedEndpointPath,
	)
	reloadEndpointRequest, requestCreateError := http.NewRequestWithContext(
		reloadContext,
		http.MethodPost,
		reloadURL,
		nil,
	)
	if requestCreateError != nil {
		return fmt.Errorf("create request: %w", requestCreateError)
	}
	applyFrameworkRuntimeReloadRequestHeaders(
		reloadEndpointRequest,
		reloadRequest,
	)

	reloadEndpointResponse, requestError := (&http.Client{}).Do(
		reloadEndpointRequest,
	)
	if requestError != nil {
		return fmt.Errorf("request failed: %w", requestError)
	}
	defer reloadEndpointResponse.Body.Close()
	if reloadEndpointResponse.StatusCode != http.StatusOK {
		return fmt.Errorf(
			"endpoint returned %d",
			reloadEndpointResponse.StatusCode,
		)
	}

	return nil
}

func applyFrameworkRuntimeReloadRequestHeaders(
	request *http.Request,
	reloadRequest wave.FrameworkRuntimeReloadRequest,
) {
	if request == nil {
		return
	}

	trimmedReloadAttemptID := strings.TrimSpace(reloadRequest.ReloadAttemptID)
	if trimmedReloadAttemptID != "" {
		request.Header.Set(
			wave.FrameworkRuntimeReloadAttemptIDHeaderName,
			trimmedReloadAttemptID,
		)
	}

	trimmedExpectedBuildID := strings.TrimSpace(reloadRequest.ExpectedBuildID)
	if trimmedExpectedBuildID != "" {
		request.Header.Set(
			wave.FrameworkRuntimeReloadExpectedBuildIDHeaderName,
			trimmedExpectedBuildID,
		)
	}

	trimmedReloadTrigger := strings.TrimSpace(reloadRequest.ReloadTrigger)
	if trimmedReloadTrigger != "" {
		request.Header.Set(
			wave.FrameworkRuntimeReloadTriggerHeaderName,
			trimmedReloadTrigger,
		)
	}
}

// ExecuteFrameworkRuntimeReloadRequestsWithContext executes one or more runtime
// reload endpoint requests and returns on first failure.
func ExecuteFrameworkRuntimeReloadRequestsWithContext(
	reloadContext context.Context,
	appPort int,
	reloadRequests []wave.FrameworkRuntimeReloadRequest,
) error {
	for _, reloadRequest := range reloadRequests {
		reloadError := CallFrameworkRuntimeReloadEndpointWithContext(
			reloadContext,
			appPort,
			reloadRequest,
		)
		if reloadError == nil {
			continue
		}
		return fmt.Errorf(
			"framework runtime reload request failed (endpoint=%q attempt=%q expected_build_id=%q trigger=%q): %w",
			strings.TrimSpace(reloadRequest.EndpointPath),
			strings.TrimSpace(reloadRequest.ReloadAttemptID),
			strings.TrimSpace(reloadRequest.ExpectedBuildID),
			strings.TrimSpace(reloadRequest.ReloadTrigger),
			reloadError,
		)
	}
	return nil
}

// RefreshRuntimeState captures running refresh server resources.
type RefreshRuntimeState struct {
	Manager *broadcast.Manager
	Server  *http.Server
	Port    int
	Cancel  context.CancelFunc
}

// RefreshRuntimeStartResult contains started refresh server state plus runtime
// runners.
type RefreshRuntimeStartResult struct {
	State RefreshRuntimeState

	RunManager func()
	RunServer  func()
}

// StartRefreshRuntime initializes refresh server resources and returns runners
// that should be launched by the caller.
func StartRefreshRuntime(
	preferredPort int,
	log *slog.Logger,
) (RefreshRuntimeStartResult, error) {
	listenOnPort := func(port int) (net.Listener, error) {
		return net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	}

	listener, listenError := listenOnPort(preferredPort)
	if listenError != nil && preferredPort > 0 {
		fallbackListener, fallbackListenError := listenOnPort(0)
		if fallbackListenError != nil {
			return RefreshRuntimeStartResult{}, fmt.Errorf(
				"listen refresh server on preferred port %d: %w (fallback listen failed: %v)",
				preferredPort,
				listenError,
				fallbackListenError,
			)
		}
		listener = fallbackListener
		listenError = nil
	}
	if listenError != nil {
		return RefreshRuntimeStartResult{}, fmt.Errorf(
			"listen refresh server: %w",
			listenError,
		)
	}
	actualPort := listener.Addr().(*net.TCPAddr).Port
	refreshManager := broadcast.NewManager(log, broadcast.ManagerConfig{})

	refreshServerContext, cancelRefreshServer := context.WithCancel(
		context.Background(),
	)
	refreshServer := &http.Server{
		Addr:    listener.Addr().String(),
		Handler: NewRefreshRuntimeMux(refreshManager),
	}

	return RefreshRuntimeStartResult{
		State: RefreshRuntimeState{
			Manager: refreshManager,
			Server:  refreshServer,
			Port:    actualPort,
			Cancel:  cancelRefreshServer,
		},
		RunManager: func() {
			refreshManager.Run(refreshServerContext)
		},
		RunServer: func() {
			if serveError := refreshServer.Serve(listener); serveError != nil &&
				!errors.Is(serveError, http.ErrServerClosed) {
				if log != nil {
					log.Error(
						"refresh server serve failed",
						"error",
						serveError,
					)
				}
			}
		},
	}, nil
}

// StopRefreshRuntime terminates refresh server and manager resources.
func StopRefreshRuntime(refreshRuntimeState RefreshRuntimeState) {
	if refreshRuntimeState.Cancel != nil {
		refreshRuntimeState.Cancel()
	}
	if refreshRuntimeState.Server != nil {
		_ = refreshRuntimeState.Server.Close()
	}
	if refreshRuntimeState.Manager != nil {
		refreshRuntimeState.Manager.Close()
	}
}

// NewRefreshRuntimeMux builds the refresh server endpoint mux.
func NewRefreshRuntimeMux(refreshManager *broadcast.Manager) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc(
		"/events",
		func(responseWriter http.ResponseWriter, request *http.Request) {
			responseWriter.Header().Set("Access-Control-Allow-Origin", "*")
			responseWriter.Header().Set(
				"Access-Control-Allow-Methods",
				"GET, OPTIONS",
			)
			if request.Method == http.MethodOptions {
				responseWriter.WriteHeader(http.StatusNoContent)
				return
			}
			refreshManager.ServeHTTP(responseWriter, request)
		},
	)
	mux.Handle("/refresh", refreshManager)
	mux.HandleFunc(
		"/get-refresh-script-inner",
		func(responseWriter http.ResponseWriter, _ *http.Request) {
			responseWriter.Header().Set("Content-Type", "text/plain")
			responseWriter.WriteHeader(http.StatusOK)
			_, _ = responseWriter.Write(
				[]byte("// wave refresh script placeholder"),
			)
		},
	)
	mux.HandleFunc(
		"/healthz",
		func(responseWriter http.ResponseWriter, _ *http.Request) {
			responseWriter.WriteHeader(http.StatusOK)
			_, _ = responseWriter.Write([]byte("ok"))
		},
	)
	return mux
}

// ResolveRefreshRuntimePortFromEnvironmentOrDefault resolves refresh port from
// env.
func ResolveRefreshRuntimePortFromEnvironmentOrDefault(defaultPort int) int {
	fromEnvironment := strings.TrimSpace(os.Getenv("WAVE_REFRESH_PORT"))
	if fromEnvironment == "" {
		return defaultPort
	}
	parsedPort, parseError := strconv.Atoi(fromEnvironment)
	if parseError != nil || parsedPort <= 0 {
		return defaultPort
	}
	return parsedPort
}

// ResolveDirectoryPathFromFilePath returns the cleaned parent directory for a
// path.
func ResolveDirectoryPathFromFilePath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	return filepath.Clean(filepath.Dir(path))
}
