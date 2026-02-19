package tooling

import (
	"context"
	"encoding/base64"
	"github.com/vormadev/vorma/wave/tooling/builder"
	"github.com/vormadev/vorma/wave/tooling/devserver"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/vormadev/vorma/lab/vitecmd"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/tooling/broadcast"
)

func TestBroadcastRebuilding_SendsPayloadWhenEnabled(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false

	s := &devserver.Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
		RefreshMgr: &broadcast.Manager{
			Broadcast: make(chan broadcast.Payload, 1),
		},
		RefreshMgrCtx: context.Background(),
	}

	s.BroadcastRebuilding()

	select {
	case msg := <-s.RefreshMgr.Broadcast:
		if msg.ChangeType != broadcast.ChangeTypeRebuilding {
			t.Fatalf("expected rebuilding payload, got %#v", msg)
		}
	default:
		t.Fatal("expected rebuilding broadcast payload, got none")
	}
}

func TestBroadcastRebuilding_NoOpInServerOnlyMode(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true

	s := &devserver.Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
		RefreshMgr: &broadcast.Manager{
			Broadcast: make(chan broadcast.Payload, 1),
		},
		RefreshMgrCtx: context.Background(),
	}

	s.BroadcastRebuilding()

	select {
	case msg := <-s.RefreshMgr.Broadcast:
		t.Fatalf("did not expect rebuilding payload in server-only mode, got %#v", msg)
	default:
	}
}

func TestBroadcastRebuilding_NoOpWhenContextCanceled(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s := &devserver.Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
		RefreshMgr: &broadcast.Manager{
			Broadcast: make(chan broadcast.Payload, 1),
		},
		RefreshMgrCtx: ctx,
	}

	s.BroadcastRebuilding()

	select {
	case msg := <-s.RefreshMgr.Broadcast:
		t.Fatalf("did not expect rebuilding payload after context cancellation, got %#v", msg)
	default:
	}
}

func TestBroadcastReload_StopsWhenContextCanceled(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s := &devserver.Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
		RefreshMgr: &broadcast.Manager{
			Broadcast: make(chan broadcast.Payload, 1),
		},
		RefreshMgrCtx: ctx,
	}

	reloadOutcome := s.BroadcastReload(devserver.ReloadOpts{
		Payload: broadcast.Payload{ChangeType: broadcast.ChangeTypeOther},
	})
	if !reloadOutcome.BroadcastEnabled {
		t.Fatalf("expected broadcast to be enabled, got %#v", reloadOutcome)
	}
	if reloadOutcome.BroadcastContextActive {
		t.Fatalf("expected canceled context to mark outcome inactive, got %#v", reloadOutcome)
	}
	if reloadOutcome.PayloadBroadcasted {
		t.Fatalf("expected no payload broadcast after cancellation, got %#v", reloadOutcome)
	}

	select {
	case msg := <-s.RefreshMgr.Broadcast:
		t.Fatalf("did not expect payload after cancellation, got %#v", msg)
	default:
	}
}

func TestShouldBroadcastReloadPayloadAfterReadiness(t *testing.T) {
	testCases := []struct {
		Name             string
		ReloadOptions    devserver.ReloadOpts
		CycleViteApplied bool
		Expected         bool
	}{
		{
			Name:             "non-cycle reload broadcasts payload",
			ReloadOptions:    devserver.ReloadOpts{CycleVite: false},
			CycleViteApplied: false,
			Expected:         true,
		},
		{
			Name:             "cycle requested and applied skips payload",
			ReloadOptions:    devserver.ReloadOpts{CycleVite: true},
			CycleViteApplied: true,
			Expected:         false,
		},
		{
			Name:             "cycle requested but not applied broadcasts payload",
			ReloadOptions:    devserver.ReloadOpts{CycleVite: true},
			CycleViteApplied: false,
			Expected:         true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			got := devserver.ShouldBroadcastReloadPayloadAfterReadiness(
				testCase.ReloadOptions,
				testCase.CycleViteApplied,
			)
			if got != testCase.Expected {
				t.Fatalf(
					"devserver.ShouldBroadcastReloadPayloadAfterReadiness(%#v, %t)=%t, want %t",
					testCase.ReloadOptions,
					testCase.CycleViteApplied,
					got,
					testCase.Expected,
				)
			}
		})
	}
}

func TestBroadcastReload_CycleViteWithoutActiveContextFallsBackToPayloadBroadcast(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false
	cfg.Vite = &wave.ViteConfig{
		JSPackageManagerBaseCmd: "echo",
		DefaultPort:             5209,
	}

	s := &devserver.Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
		RefreshMgr: &broadcast.Manager{
			Broadcast: make(chan broadcast.Payload, 1),
		},
		RefreshMgrCtx: context.Background(),
	}

	reloadOutcome := s.BroadcastReload(devserver.ReloadOpts{
		Payload:   broadcast.Payload{ChangeType: broadcast.ChangeTypeOther},
		CycleVite: true,
	})
	if !reloadOutcome.BroadcastEnabled || !reloadOutcome.BroadcastContextActive {
		t.Fatalf("expected active broadcast context, got %#v", reloadOutcome)
	}
	if reloadOutcome.ReadinessOutcome.CycleViteApplied {
		t.Fatalf("expected cycle-vite to be unapplied without active vite context, got %#v", reloadOutcome)
	}
	if !reloadOutcome.ShouldBroadcastPayload || !reloadOutcome.PayloadBroadcasted {
		t.Fatalf("expected payload fallback broadcast outcome, got %#v", reloadOutcome)
	}

	select {
	case msg := <-s.RefreshMgr.Broadcast:
		if msg.ChangeType != broadcast.ChangeTypeOther {
			t.Fatalf("expected fallback hard reload payload, got %#v", msg)
		}
	default:
		t.Fatal("expected fallback broadcast payload when cycleVite cannot be applied")
	}
}

func TestBroadcastReload_CycleViteFailureFallsBackToPayloadBroadcast(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false
	cfg.Vite = &wave.ViteConfig{
		JSPackageManagerBaseCmd: "command_that_does_not_exist_for_cycle_vite_failure_test",
		DefaultPort:             5211,
	}

	builder := toolingbuilder.NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &devserver.Server{
		Cfg:     cfg,
		Log:     newDiscardLogger(),
		Builder: builder,
		ViteCtx: vitecmd.NewBuildCtx(&vitecmd.BuildCtxOptions{
			DefaultPort: cfg.Vite.DefaultPort,
		}),
		RefreshMgr: &broadcast.Manager{
			Broadcast: make(chan broadcast.Payload, 1),
		},
		RefreshMgrCtx: context.Background(),
	}

	reloadOutcome := s.BroadcastReload(devserver.ReloadOpts{
		Payload:   broadcast.Payload{ChangeType: broadcast.ChangeTypeOther},
		CycleVite: true,
	})
	if !reloadOutcome.BroadcastEnabled || !reloadOutcome.BroadcastContextActive {
		t.Fatalf("expected active broadcast context, got %#v", reloadOutcome)
	}
	if reloadOutcome.ReadinessOutcome.CycleViteApplied {
		t.Fatalf("expected failed cycle-vite to be reported as unapplied, got %#v", reloadOutcome)
	}
	if !reloadOutcome.ShouldBroadcastPayload || !reloadOutcome.PayloadBroadcasted {
		t.Fatalf("expected payload fallback broadcast after cycle failure, got %#v", reloadOutcome)
	}

	select {
	case msg := <-s.RefreshMgr.Broadcast:
		if msg.ChangeType != broadcast.ChangeTypeOther {
			t.Fatalf("expected fallback hard reload payload, got %#v", msg)
		}
	default:
		t.Fatal("expected fallback broadcast payload when cycleVite restart fails")
	}
}

func TestExecuteBrowserPhase_InvalidateViteFallbackWithoutViteSetsHardReload(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false
	cfg.Vite = nil

	s := &devserver.Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
	}

	work := &devserver.WorkSet{
		Browser: devserver.BrowserPhaseDecision{
			Action: devserver.BrowserPhaseActionInvalidateVite,
		},
	}
	s.ExecuteBrowserPhase(work)

	if work.Browser.Action != devserver.BrowserPhaseActionHardReload || !work.Browser.WaitForApp {
		t.Fatalf(
			"expected invalidate fallback to set reload+waitApp, got reload=%v waitApp=%v",
			work.Browser.Action == devserver.BrowserPhaseActionHardReload,
			work.Browser.WaitForApp,
		)
	}
	if work.Browser.WaitForVite {
		t.Fatal("did not expect waitForVite when Vite is disabled")
	}
}

func TestExecuteBrowserPhase_InvalidateViteFailureFallsBackToHardReload(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false
	cfg.Vite = &wave.ViteConfig{JSPackageManagerBaseCmd: "pnpm"}

	s := &devserver.Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
	}

	work := &devserver.WorkSet{
		Browser: devserver.BrowserPhaseDecision{
			Action: devserver.BrowserPhaseActionInvalidateVite,
		},
	}
	s.ExecuteBrowserPhase(work)

	if work.Browser.Action != devserver.BrowserPhaseActionHardReload ||
		!work.Browser.WaitForApp ||
		!work.Browser.WaitForVite {
		t.Fatalf(
			"expected fallback hard reload flags, got reload=%v waitApp=%v waitVite=%v",
			work.Browser.Action == devserver.BrowserPhaseActionHardReload,
			work.Browser.WaitForApp,
			work.Browser.WaitForVite,
		)
	}
}

func TestExecuteBrowserPhase_HotReloadCSSBroadcastsCriticalAndNormalPayloads(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false

	if err := toolingbuilder.SetupDistDir(cfg); err != nil {
		t.Fatalf("SetupDistDir returned error: %v", err)
	}

	if err := os.WriteFile(cfg.Dist.CriticalCSS(), []byte("body{color:red;}"), 0644); err != nil {
		t.Fatalf("failed writing critical CSS: %v", err)
	}
	if err := os.WriteFile(cfg.Dist.NormalCSSRef(), []byte("styles.css"), 0644); err != nil {
		t.Fatalf("failed writing normal CSS ref: %v", err)
	}

	builder := toolingbuilder.NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &devserver.Server{
		Cfg:     cfg,
		Log:     newDiscardLogger(),
		Builder: builder,
		RefreshMgr: &broadcast.Manager{
			Broadcast: make(chan broadcast.Payload, 2),
		},
		RefreshMgrCtx: context.Background(),
	}

	work := &devserver.WorkSet{
		Browser: devserver.BrowserPhaseDecision{
			Action: devserver.BrowserPhaseActionHotReloadCSS,
		},
		Build: devserver.BuildPhaseDecision{
			BuildCriticalCSS: true,
			BuildNormalCSS:   true,
		},
	}
	s.ExecuteBrowserPhase(work)

	first := <-s.RefreshMgr.Broadcast
	second := <-s.RefreshMgr.Broadcast

	if first.ChangeType != broadcast.ChangeTypeCriticalCSS {
		t.Fatalf("expected first payload to be critical CSS reload, got %#v", first)
	}
	if second.ChangeType != broadcast.ChangeTypeNormalCSS {
		t.Fatalf("expected second payload to be normal CSS reload, got %#v", second)
	}

	expectedCritical := base64.StdEncoding.EncodeToString([]byte("body{color:red;}"))
	if first.CriticalCSS != expectedCritical {
		t.Fatalf("unexpected critical CSS Payload: %q", first.CriticalCSS)
	}

	expectedNormalURL := filepath.ToSlash(cfg.PublicPathPrefix() + "styles.css")
	if second.NormalCSSURL != expectedNormalURL {
		t.Fatalf("unexpected normal CSS URL Payload: %q", second.NormalCSSURL)
	}
}

func TestExecuteBrowserPhase_HotReloadCSSSkipsPayloadsWhenFreshBuildOutputsAreUnavailable(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Core.CSSEntryFiles = wave.CSSEntryFiles{
		Critical:    filepath.Join(root, "styles", "critical.css"),
		NonCritical: filepath.Join(root, "styles", "normal.css"),
	}
	cfg.Dist = wave.DistLayout{Root: cfg.Core.DistDir}

	if err := os.MkdirAll(filepath.Dir(cfg.Core.CSSEntryFiles.Critical), 0o755); err != nil {
		t.Fatalf("failed creating critical css entry parent dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Core.CSSEntryFiles.NonCritical), 0o755); err != nil {
		t.Fatalf("failed creating normal css entry parent dir: %v", err)
	}
	if err := os.WriteFile(cfg.Core.CSSEntryFiles.Critical, []byte(`body { color: red; }`), 0o644); err != nil {
		t.Fatalf("failed writing critical css entry file: %v", err)
	}
	if err := os.WriteFile(cfg.Core.CSSEntryFiles.NonCritical, []byte(`body { color: blue; }`), 0o644); err != nil {
		t.Fatalf("failed writing normal css entry file: %v", err)
	}

	builder := toolingbuilder.NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	if err := builder.BuildCriticalCSS(true); err != nil {
		t.Fatalf("initial BuildCriticalCSS returned error: %v", err)
	}
	if err := builder.BuildNormalCSS(true); err != nil {
		t.Fatalf("initial BuildNormalCSS returned error: %v", err)
	}

	cfg.Core.CSSEntryFiles.Critical = filepath.Join(root, "styles", "missing-critical.css")
	cfg.Core.CSSEntryFiles.NonCritical = filepath.Join(root, "styles", "missing-normal.css")
	if err := builder.BuildCriticalCSS(true); err == nil {
		t.Fatal("expected BuildCriticalCSS to fail for missing entry")
	}
	if err := builder.BuildNormalCSS(true); err == nil {
		t.Fatal("expected BuildNormalCSS to fail for missing entry")
	}

	s := &devserver.Server{
		Cfg:     cfg,
		Log:     newDiscardLogger(),
		Builder: builder,
		RefreshMgr: &broadcast.Manager{
			Broadcast: make(chan broadcast.Payload, 2),
		},
		RefreshMgrCtx: context.Background(),
	}

	work := &devserver.WorkSet{
		Browser: devserver.BrowserPhaseDecision{
			Action: devserver.BrowserPhaseActionHotReloadCSS,
		},
		Build: devserver.BuildPhaseDecision{
			BuildCriticalCSS: true,
			BuildNormalCSS:   true,
		},
	}
	s.ExecuteBrowserPhase(work)

	select {
	case payload := <-s.RefreshMgr.Broadcast:
		t.Fatalf("did not expect css hot-reload payload after failed rebuilds, got %#v", payload)
	default:
	}
}

func TestExecuteBrowserPhase_RevalidateBroadcastsRevalidatePayload(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false

	s := &devserver.Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
		RefreshMgr: &broadcast.Manager{
			Broadcast: make(chan broadcast.Payload, 1),
		},
		RefreshMgrCtx: context.Background(),
	}

	work := &devserver.WorkSet{
		Browser: devserver.BrowserPhaseDecision{
			Action: devserver.BrowserPhaseActionRevalidate,
		},
	}
	s.ExecuteBrowserPhase(work)

	select {
	case msg := <-s.RefreshMgr.Broadcast:
		if msg.ChangeType != broadcast.ChangeTypeRevalidate {
			t.Fatalf("expected revalidate payload, got %#v", msg)
		}
	default:
		t.Fatal("expected revalidate broadcast payload, got none")
	}
}

func TestExecuteBrowserPhase_HotReloadCSSBroadcastsCriticalOnlyPayload(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false

	if err := toolingbuilder.SetupDistDir(cfg); err != nil {
		t.Fatalf("SetupDistDir returned error: %v", err)
	}
	if err := os.WriteFile(cfg.Dist.CriticalCSS(), []byte("body{background:black;}"), 0644); err != nil {
		t.Fatalf("failed writing critical CSS: %v", err)
	}

	builder := toolingbuilder.NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &devserver.Server{
		Cfg:     cfg,
		Log:     newDiscardLogger(),
		Builder: builder,
		RefreshMgr: &broadcast.Manager{
			Broadcast: make(chan broadcast.Payload, 1),
		},
		RefreshMgrCtx: context.Background(),
	}

	work := &devserver.WorkSet{
		Browser: devserver.BrowserPhaseDecision{
			Action: devserver.BrowserPhaseActionHotReloadCSS,
		},
		Build: devserver.BuildPhaseDecision{
			BuildCriticalCSS: true,
		},
	}
	s.ExecuteBrowserPhase(work)

	select {
	case msg := <-s.RefreshMgr.Broadcast:
		if msg.ChangeType != broadcast.ChangeTypeCriticalCSS {
			t.Fatalf("expected critical css payload, got %#v", msg)
		}
	default:
		t.Fatal("expected critical css payload, got none")
	}
}

func TestExecuteBrowserPhase_HotReloadCSSBroadcastsNormalOnlyPayload(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false

	if err := toolingbuilder.SetupDistDir(cfg); err != nil {
		t.Fatalf("SetupDistDir returned error: %v", err)
	}
	if err := os.WriteFile(cfg.Dist.NormalCSSRef(), []byte("styles-extra.css"), 0644); err != nil {
		t.Fatalf("failed writing normal css ref: %v", err)
	}

	builder := toolingbuilder.NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &devserver.Server{
		Cfg:     cfg,
		Log:     newDiscardLogger(),
		Builder: builder,
		RefreshMgr: &broadcast.Manager{
			Broadcast: make(chan broadcast.Payload, 1),
		},
		RefreshMgrCtx: context.Background(),
	}

	work := &devserver.WorkSet{
		Browser: devserver.BrowserPhaseDecision{
			Action: devserver.BrowserPhaseActionHotReloadCSS,
		},
		Build: devserver.BuildPhaseDecision{
			BuildNormalCSS: true,
		},
	}
	s.ExecuteBrowserPhase(work)

	select {
	case msg := <-s.RefreshMgr.Broadcast:
		if msg.ChangeType != broadcast.ChangeTypeNormalCSS {
			t.Fatalf("expected normal css payload, got %#v", msg)
		}
	default:
		t.Fatal("expected normal css payload, got none")
	}
}

func TestExecuteBrowserPhase_NoOpWhenServerOnlyMode(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true

	s := &devserver.Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
		RefreshMgr: &broadcast.Manager{
			Broadcast: make(chan broadcast.Payload, 1),
		},
		RefreshMgrCtx: context.Background(),
	}

	work := &devserver.WorkSet{
		Browser: devserver.BrowserPhaseDecision{
			Action: devserver.BrowserPhaseActionHardReload,
		},
	}
	s.ExecuteBrowserPhase(work)

	select {
	case msg := <-s.RefreshMgr.Broadcast:
		t.Fatalf("did not expect browser broadcast in server-only mode, got %#v", msg)
	default:
	}
}

func TestExecuteBrowserPhase_InvalidateViteSuccessReturnsWithoutReloadFallback(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false
	cfg.Vite = &wave.ViteConfig{JSPackageManagerBaseCmd: "pnpm"}

	invalidateServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/__vorma_invalidate_filemap" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer invalidateServer.Close()

	parsedURL, err := url.Parse(invalidateServer.URL)
	if err != nil {
		t.Fatalf("failed parsing invalidate test server URL: %v", err)
	}
	port, err := strconv.Atoi(parsedURL.Port())
	if err != nil {
		t.Fatalf("failed parsing invalidate test server port: %v", err)
	}

	s := &devserver.Server{
		Cfg:        cfg,
		Log:        newDiscardLogger(),
		ViteCtx:    vitecmd.NewBuildCtx(&vitecmd.BuildCtxOptions{DefaultPort: port}),
		RefreshMgr: &broadcast.Manager{Broadcast: make(chan broadcast.Payload, 1)},
		// context must be alive so broadcastReload would run if fallback occurred
		RefreshMgrCtx: context.Background(),
	}

	work := &devserver.WorkSet{
		Browser: devserver.BrowserPhaseDecision{
			Action: devserver.BrowserPhaseActionInvalidateVite,
		},
	}
	s.ExecuteBrowserPhase(work)

	if work.Browser.Action != devserver.BrowserPhaseActionInvalidateVite ||
		work.Browser.WaitForApp ||
		work.Browser.WaitForVite {
		t.Fatalf(
			"expected no fallback flags on successful vite invalidation, got reload=%v waitApp=%v waitVite=%v",
			work.Browser.Action == devserver.BrowserPhaseActionHardReload,
			work.Browser.WaitForApp,
			work.Browser.WaitForVite,
		)
	}

	select {
	case msg := <-s.RefreshMgr.Broadcast:
		t.Fatalf("did not expect fallback reload broadcast on successful invalidation, got %#v", msg)
	default:
	}
}
