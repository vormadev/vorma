package tooling

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

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

	s.broadcastReload(reloadOpts{
		payload: refreshPayload{ChangeType: changeTypeOther},
	})

	select {
	case msg := <-s.refreshMgr.broadcast:
		t.Fatalf("did not expect payload after cancellation, got %#v", msg)
	default:
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

	work := &workSet{invalidateVite: true}
	s.executeBrowserPhase(work)

	if !work.reloadBrowser || !work.waitForApp {
		t.Fatalf(
			"expected invalidate fallback to set reload+waitApp, got reload=%v waitApp=%v",
			work.reloadBrowser,
			work.waitForApp,
		)
	}
	if work.waitForVite {
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

	work := &workSet{invalidateVite: true}
	s.executeBrowserPhase(work)

	if !work.reloadBrowser || !work.waitForApp || !work.waitForVite {
		t.Fatalf(
			"expected fallback hard reload flags, got reload=%v waitApp=%v waitVite=%v",
			work.reloadBrowser,
			work.waitForApp,
			work.waitForVite,
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
		hotReloadCSS:     true,
		buildCriticalCSS: true,
		buildNormalCSS:   true,
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
