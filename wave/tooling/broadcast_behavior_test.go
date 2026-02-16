package tooling

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/vormadev/vorma/lab/vitecmd"
	"github.com/vormadev/vorma/wave"
)

func TestBroadcastRebuilding_SendsPayloadWhenEnabled(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false

	s := &server{
		cfg: cfg,
		log: newDiscardLogger(),
		refreshMgr: &clientManager{
			broadcast: make(chan refreshPayload, 1),
		},
		refreshMgrCtx: context.Background(),
	}

	s.broadcastRebuilding()

	select {
	case msg := <-s.refreshMgr.broadcast:
		if msg.ChangeType != changeTypeRebuilding {
			t.Fatalf("expected rebuilding payload, got %#v", msg)
		}
	default:
		t.Fatal("expected rebuilding broadcast payload, got none")
	}
}

func TestBroadcastRebuilding_NoOpInServerOnlyMode(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true

	s := &server{
		cfg: cfg,
		log: newDiscardLogger(),
		refreshMgr: &clientManager{
			broadcast: make(chan refreshPayload, 1),
		},
		refreshMgrCtx: context.Background(),
	}

	s.broadcastRebuilding()

	select {
	case msg := <-s.refreshMgr.broadcast:
		t.Fatalf("did not expect rebuilding payload in server-only mode, got %#v", msg)
	default:
	}
}

func TestBroadcastRebuilding_NoOpWhenContextCanceled(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s := &server{
		cfg: cfg,
		log: newDiscardLogger(),
		refreshMgr: &clientManager{
			broadcast: make(chan refreshPayload, 1),
		},
		refreshMgrCtx: ctx,
	}

	s.broadcastRebuilding()

	select {
	case msg := <-s.refreshMgr.broadcast:
		t.Fatalf("did not expect rebuilding payload after context cancellation, got %#v", msg)
	default:
	}
}

func TestBroadcastReload_StopsWhenContextCanceled(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s := &server{
		cfg: cfg,
		log: newDiscardLogger(),
		refreshMgr: &clientManager{
			broadcast: make(chan refreshPayload, 1),
		},
		refreshMgrCtx: ctx,
	}

	reloadOutcome := s.broadcastReload(reloadOpts{
		payload: refreshPayload{ChangeType: changeTypeOther},
	})
	if !reloadOutcome.broadcastEnabled {
		t.Fatalf("expected broadcast to be enabled, got %#v", reloadOutcome)
	}
	if reloadOutcome.broadcastContextActive {
		t.Fatalf("expected canceled context to mark outcome inactive, got %#v", reloadOutcome)
	}
	if reloadOutcome.payloadBroadcasted {
		t.Fatalf("expected no payload broadcast after cancellation, got %#v", reloadOutcome)
	}

	select {
	case msg := <-s.refreshMgr.broadcast:
		t.Fatalf("did not expect payload after cancellation, got %#v", msg)
	default:
	}
}

func TestShouldBroadcastReloadPayloadAfterReadiness(t *testing.T) {
	testCases := []struct {
		name             string
		reloadOptions    reloadOpts
		cycleViteApplied bool
		expected         bool
	}{
		{
			name:             "non-cycle reload broadcasts payload",
			reloadOptions:    reloadOpts{cycleVite: false},
			cycleViteApplied: false,
			expected:         true,
		},
		{
			name:             "cycle requested and applied skips payload",
			reloadOptions:    reloadOpts{cycleVite: true},
			cycleViteApplied: true,
			expected:         false,
		},
		{
			name:             "cycle requested but not applied broadcasts payload",
			reloadOptions:    reloadOpts{cycleVite: true},
			cycleViteApplied: false,
			expected:         true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := shouldBroadcastReloadPayloadAfterReadiness(
				testCase.reloadOptions,
				testCase.cycleViteApplied,
			)
			if got != testCase.expected {
				t.Fatalf(
					"shouldBroadcastReloadPayloadAfterReadiness(%#v, %t)=%t, want %t",
					testCase.reloadOptions,
					testCase.cycleViteApplied,
					got,
					testCase.expected,
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

	s := &server{
		cfg: cfg,
		log: newDiscardLogger(),
		refreshMgr: &clientManager{
			broadcast: make(chan refreshPayload, 1),
		},
		refreshMgrCtx: context.Background(),
	}

	reloadOutcome := s.broadcastReload(reloadOpts{
		payload:   refreshPayload{ChangeType: changeTypeOther},
		cycleVite: true,
	})
	if !reloadOutcome.broadcastEnabled || !reloadOutcome.broadcastContextActive {
		t.Fatalf("expected active broadcast context, got %#v", reloadOutcome)
	}
	if reloadOutcome.readinessOutcome.cycleViteApplied {
		t.Fatalf("expected cycle-vite to be unapplied without active vite context, got %#v", reloadOutcome)
	}
	if !reloadOutcome.shouldBroadcastPayload || !reloadOutcome.payloadBroadcasted {
		t.Fatalf("expected payload fallback broadcast outcome, got %#v", reloadOutcome)
	}

	select {
	case msg := <-s.refreshMgr.broadcast:
		if msg.ChangeType != changeTypeOther {
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

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &server{
		cfg:     cfg,
		log:     newDiscardLogger(),
		builder: builder,
		viteCtx: vitecmd.NewBuildCtx(&vitecmd.BuildCtxOptions{
			DefaultPort: cfg.Vite.DefaultPort,
		}),
		refreshMgr: &clientManager{
			broadcast: make(chan refreshPayload, 1),
		},
		refreshMgrCtx: context.Background(),
	}

	reloadOutcome := s.broadcastReload(reloadOpts{
		payload:   refreshPayload{ChangeType: changeTypeOther},
		cycleVite: true,
	})
	if !reloadOutcome.broadcastEnabled || !reloadOutcome.broadcastContextActive {
		t.Fatalf("expected active broadcast context, got %#v", reloadOutcome)
	}
	if reloadOutcome.readinessOutcome.cycleViteApplied {
		t.Fatalf("expected failed cycle-vite to be reported as unapplied, got %#v", reloadOutcome)
	}
	if !reloadOutcome.shouldBroadcastPayload || !reloadOutcome.payloadBroadcasted {
		t.Fatalf("expected payload fallback broadcast after cycle failure, got %#v", reloadOutcome)
	}

	select {
	case msg := <-s.refreshMgr.broadcast:
		if msg.ChangeType != changeTypeOther {
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

	s := &server{
		cfg: cfg,
		log: newDiscardLogger(),
	}

	work := &workSet{
		browser: browserPhaseDecision{
			action: browserPhaseActionInvalidateVite,
		},
	}
	s.executeBrowserPhase(work)

	if work.browser.action != browserPhaseActionHardReload || !work.browser.waitForApp {
		t.Fatalf(
			"expected invalidate fallback to set reload+waitApp, got reload=%v waitApp=%v",
			work.browser.action == browserPhaseActionHardReload,
			work.browser.waitForApp,
		)
	}
	if work.browser.waitForVite {
		t.Fatal("did not expect waitForVite when Vite is disabled")
	}
}

func TestExecuteBrowserPhase_InvalidateViteFailureFallsBackToHardReload(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false
	cfg.Vite = &wave.ViteConfig{JSPackageManagerBaseCmd: "pnpm"}

	s := &server{
		cfg: cfg,
		log: newDiscardLogger(),
	}

	work := &workSet{
		browser: browserPhaseDecision{
			action: browserPhaseActionInvalidateVite,
		},
	}
	s.executeBrowserPhase(work)

	if work.browser.action != browserPhaseActionHardReload ||
		!work.browser.waitForApp ||
		!work.browser.waitForVite {
		t.Fatalf(
			"expected fallback hard reload flags, got reload=%v waitApp=%v waitVite=%v",
			work.browser.action == browserPhaseActionHardReload,
			work.browser.waitForApp,
			work.browser.waitForVite,
		)
	}
}

func TestExecuteBrowserPhase_HotReloadCSSBroadcastsCriticalAndNormalPayloads(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false

	if err := SetupDistDir(cfg); err != nil {
		t.Fatalf("SetupDistDir returned error: %v", err)
	}

	if err := os.WriteFile(cfg.Dist.CriticalCSS(), []byte("body{color:red;}"), 0644); err != nil {
		t.Fatalf("failed writing critical CSS: %v", err)
	}
	if err := os.WriteFile(cfg.Dist.NormalCSSRef(), []byte("styles.css"), 0644); err != nil {
		t.Fatalf("failed writing normal CSS ref: %v", err)
	}

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &server{
		cfg:     cfg,
		log:     newDiscardLogger(),
		builder: builder,
		refreshMgr: &clientManager{
			broadcast: make(chan refreshPayload, 2),
		},
		refreshMgrCtx: context.Background(),
	}

	work := &workSet{
		browser: browserPhaseDecision{
			action: browserPhaseActionHotReloadCSS,
		},
		build: buildPhaseDecision{
			buildCriticalCSS: true,
			buildNormalCSS:   true,
		},
	}
	s.executeBrowserPhase(work)

	first := <-s.refreshMgr.broadcast
	second := <-s.refreshMgr.broadcast

	if first.ChangeType != changeTypeCriticalCSS {
		t.Fatalf("expected first payload to be critical CSS reload, got %#v", first)
	}
	if second.ChangeType != changeTypeNormalCSS {
		t.Fatalf("expected second payload to be normal CSS reload, got %#v", second)
	}

	expectedCritical := base64.StdEncoding.EncodeToString([]byte("body{color:red;}"))
	if first.CriticalCSS != expectedCritical {
		t.Fatalf("unexpected critical CSS payload: %q", first.CriticalCSS)
	}

	expectedNormalURL := filepath.ToSlash(cfg.PublicPathPrefix() + "styles.css")
	if second.NormalCSSURL != expectedNormalURL {
		t.Fatalf("unexpected normal CSS URL payload: %q", second.NormalCSSURL)
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

	builder := NewBuilder(cfg, newDiscardLogger())
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

	s := &server{
		cfg:     cfg,
		log:     newDiscardLogger(),
		builder: builder,
		refreshMgr: &clientManager{
			broadcast: make(chan refreshPayload, 2),
		},
		refreshMgrCtx: context.Background(),
	}

	work := &workSet{
		browser: browserPhaseDecision{
			action: browserPhaseActionHotReloadCSS,
		},
		build: buildPhaseDecision{
			buildCriticalCSS: true,
			buildNormalCSS:   true,
		},
	}
	s.executeBrowserPhase(work)

	select {
	case payload := <-s.refreshMgr.broadcast:
		t.Fatalf("did not expect css hot-reload payload after failed rebuilds, got %#v", payload)
	default:
	}
}

func TestExecuteBrowserPhase_RevalidateBroadcastsRevalidatePayload(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false

	s := &server{
		cfg: cfg,
		log: newDiscardLogger(),
		refreshMgr: &clientManager{
			broadcast: make(chan refreshPayload, 1),
		},
		refreshMgrCtx: context.Background(),
	}

	work := &workSet{
		browser: browserPhaseDecision{
			action: browserPhaseActionRevalidate,
		},
	}
	s.executeBrowserPhase(work)

	select {
	case msg := <-s.refreshMgr.broadcast:
		if msg.ChangeType != changeTypeRevalidate {
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

	if err := SetupDistDir(cfg); err != nil {
		t.Fatalf("SetupDistDir returned error: %v", err)
	}
	if err := os.WriteFile(cfg.Dist.CriticalCSS(), []byte("body{background:black;}"), 0644); err != nil {
		t.Fatalf("failed writing critical CSS: %v", err)
	}

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &server{
		cfg:     cfg,
		log:     newDiscardLogger(),
		builder: builder,
		refreshMgr: &clientManager{
			broadcast: make(chan refreshPayload, 1),
		},
		refreshMgrCtx: context.Background(),
	}

	work := &workSet{
		browser: browserPhaseDecision{
			action: browserPhaseActionHotReloadCSS,
		},
		build: buildPhaseDecision{
			buildCriticalCSS: true,
		},
	}
	s.executeBrowserPhase(work)

	select {
	case msg := <-s.refreshMgr.broadcast:
		if msg.ChangeType != changeTypeCriticalCSS {
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

	if err := SetupDistDir(cfg); err != nil {
		t.Fatalf("SetupDistDir returned error: %v", err)
	}
	if err := os.WriteFile(cfg.Dist.NormalCSSRef(), []byte("styles-extra.css"), 0644); err != nil {
		t.Fatalf("failed writing normal css ref: %v", err)
	}

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &server{
		cfg:     cfg,
		log:     newDiscardLogger(),
		builder: builder,
		refreshMgr: &clientManager{
			broadcast: make(chan refreshPayload, 1),
		},
		refreshMgrCtx: context.Background(),
	}

	work := &workSet{
		browser: browserPhaseDecision{
			action: browserPhaseActionHotReloadCSS,
		},
		build: buildPhaseDecision{
			buildNormalCSS: true,
		},
	}
	s.executeBrowserPhase(work)

	select {
	case msg := <-s.refreshMgr.broadcast:
		if msg.ChangeType != changeTypeNormalCSS {
			t.Fatalf("expected normal css payload, got %#v", msg)
		}
	default:
		t.Fatal("expected normal css payload, got none")
	}
}

func TestExecuteBrowserPhase_NoOpWhenServerOnlyMode(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true

	s := &server{
		cfg: cfg,
		log: newDiscardLogger(),
		refreshMgr: &clientManager{
			broadcast: make(chan refreshPayload, 1),
		},
		refreshMgrCtx: context.Background(),
	}

	work := &workSet{
		browser: browserPhaseDecision{
			action: browserPhaseActionHardReload,
		},
	}
	s.executeBrowserPhase(work)

	select {
	case msg := <-s.refreshMgr.broadcast:
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

	s := &server{
		cfg:        cfg,
		log:        newDiscardLogger(),
		viteCtx:    vitecmd.NewBuildCtx(&vitecmd.BuildCtxOptions{DefaultPort: port}),
		refreshMgr: &clientManager{broadcast: make(chan refreshPayload, 1)},
		// context must be alive so broadcastReload would run if fallback occurred
		refreshMgrCtx: context.Background(),
	}

	work := &workSet{
		browser: browserPhaseDecision{
			action: browserPhaseActionInvalidateVite,
		},
	}
	s.executeBrowserPhase(work)

	if work.browser.action != browserPhaseActionInvalidateVite ||
		work.browser.waitForApp ||
		work.browser.waitForVite {
		t.Fatalf(
			"expected no fallback flags on successful vite invalidation, got reload=%v waitApp=%v waitVite=%v",
			work.browser.action == browserPhaseActionHardReload,
			work.browser.waitForApp,
			work.browser.waitForVite,
		)
	}

	select {
	case msg := <-s.refreshMgr.broadcast:
		t.Fatalf("did not expect fallback reload broadcast on successful invalidation, got %#v", msg)
	default:
	}
}
