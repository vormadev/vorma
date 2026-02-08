package build_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildDevControlPlaneConformance(t *testing.T) {
	devserverPath := filepath.Join(repoRoot(t), "wave", "tooling", "devserver.go")
	devserverSrc, devserverSet, devserverAST := mustParseGoSourceFile(t, devserverPath)

	lockPath := filepath.Join(repoRoot(t), "wave", "tooling", "lock.go")
	lockSrc, lockSet, lockAST := mustParseGoSourceFile(t, lockPath)

	t.Run("BDC-DEV-001_BUILD-DEV-001_single_instance_lock_is_enforced_with_stale_lock_takeover", func(t *testing.T) {
		runDevBody := mustFunctionBodySource(t, devserverSrc, devserverSet, devserverAST, "RunDev")
		for _, expected := range []string{
			"lock := newDevLock(cfg.Dist.Static())",
			"if err := lock.acquire(); err != nil",
			"defer lock.release()",
		} {
			if !strings.Contains(runDevBody, expected) {
				t.Fatalf("expected RunDev lock contract to include %q", expected)
			}
		}

		acquireBody := mustFunctionBodySource(t, lockSrc, lockSet, lockAST, "acquire")
		for _, expected := range []string{
			"data, err := os.ReadFile(l.path)",
			"if isProcessRunning(pid)",
			"return fmt.Errorf(\"%w (PID %d)\", ErrLockHeld, pid)",
			"if err := os.WriteFile(l.path, []byte(strconv.Itoa(pid)), 0644); err != nil",
		} {
			if !strings.Contains(acquireBody, expected) {
				t.Fatalf("expected lock acquire contract to include %q", expected)
			}
		}
	})

	t.Run("BDC-DEV-002_BUILD-DEV-002_refresh_server_lifecycle_persists_across_rebuild_loops", func(t *testing.T) {
		runBody := mustFunctionBodySource(t, devserverSrc, devserverSet, devserverAST, "run")
		requireOrderedSubstrings(
			t,
			runBody,
			[]string{
				"wave.MustGetPort()",
				"s.startRefreshServer(refreshPort)",
				"defer s.cleanupRefreshServer()",
				"for {",
				"s.cleanupForRebuild()",
			},
		)
	})

	t.Run("BDC-DEV-003_BUILD-DEV-003_config_reload_preserves_framework_injected_runtime_fields", func(t *testing.T) {
		reloadBody := mustFunctionBodySource(t, devserverSrc, devserverSet, devserverAST, "reloadConfig")
		for _, expected := range []string{
			"newCfg.FrameworkWatchPatterns = s.cfg.FrameworkWatchPatterns",
			"newCfg.FrameworkIgnoredPatterns = s.cfg.FrameworkIgnoredPatterns",
			"newCfg.FrameworkPublicFileMapOutDir = s.cfg.FrameworkPublicFileMapOutDir",
		} {
			if !strings.Contains(reloadBody, expected) {
				t.Fatalf("expected reloadConfig contract to include %q", expected)
			}
		}
	})

	t.Run("BDC-DEV-004_BUILD-DEV-004_sequential_go_build_mode_controls_compile_order", func(t *testing.T) {
		runBody := mustFunctionBodySource(t, devserverSrc, devserverSet, devserverAST, "run")
		requireOrderedSubstrings(
			t,
			runBody,
			[]string{
				"if recompileGo && !sequentialGo {",
				"if err := buildEg.Wait(); err != nil {",
				"if recompileGo && sequentialGo {",
			},
		)
	})

	t.Run("BDC-DEV-005_BUILD-DEV-005_build_failures_gate_retries_on_file_change_requests", func(t *testing.T) {
		runBody := mustFunctionBodySource(t, devserverSrc, devserverSet, devserverAST, "run")
		for _, expected := range []string{
			"if err := buildEg.Wait(); err != nil {",
			"s.waitForBuildRetry()",
			"if err := b.CompileGoOnly(true); err != nil {",
		} {
			if !strings.Contains(runBody, expected) {
				t.Fatalf("expected retry-gate behavior to include %q", expected)
			}
		}
	})

	t.Run("BDC-DEV-006_BUILD-DEV-006_config_restart_broadcast_occurs_before_watcher_start_for_iteration", func(t *testing.T) {
		runBody := mustFunctionBodySource(t, devserverSrc, devserverSet, devserverAST, "run")
		requireOrderedSubstrings(
			t,
			runBody,
			[]string{
				"if isConfigRestart {",
				"s.broadcastReload(reloadOpts{",
				"isConfigRestart = false",
				"close(s.watcherStartCh)",
			},
		)
	})

	t.Run("BDC-DEV-007_BUILD-DEV-007_restart_queue_uses_upgrade_and_config_priority_semantics", func(t *testing.T) {
		triggerBody := mustFunctionBodySource(t, devserverSrc, devserverSet, devserverAST, "triggerRestartWithOpts")
		for _, expected := range []string{
			"if isConfigRestart {",
			"s.restartCh <- req",
			"if pending.isConfigRestart {",
			"if recompileGo && !pending.recompileGo {",
			"s.restartCh <- pending",
		} {
			if !strings.Contains(triggerBody, expected) {
				t.Fatalf("expected restart-queue semantics to include %q", expected)
			}
		}
	})

	t.Run("BDC-DEV-008_BUILD-DEV-008_readiness_polling_is_bounded_and_requires_http_200", func(t *testing.T) {
		waitBody := mustFunctionBodySource(t, devserverSrc, devserverSet, devserverAST, "waitForReady")
		for _, expected := range []string{
			"const maxAttempts = 100",
			"const maxTotal = 10 * time.Second",
			"const requestTimeout = 500 * time.Millisecond",
			"resp, err := client.Get(url)",
			"if err == nil && resp.StatusCode == http.StatusOK {",
			"if total > maxTotal {",
		} {
			if !strings.Contains(waitBody, expected) {
				t.Fatalf("expected readiness polling contract to include %q", expected)
			}
		}
	})

	t.Run("BDC-DEV-009_BUILD-DEV-009_env_exit_override_supports_deterministic_clean_shutdown", func(t *testing.T) {
		durationBody := mustFunctionBodySource(t, devserverSrc, devserverSet, devserverAST, "getDevExitAfterDuration")
		for _, expected := range []string{
			"raw := os.Getenv(envDevExitAfterMS)",
			"ms, err := strconv.Atoi(raw)",
			"return time.Duration(ms) * time.Millisecond",
		} {
			if !strings.Contains(durationBody, expected) {
				t.Fatalf("expected dev-exit duration parser to include %q", expected)
			}
		}

		runBody := mustFunctionBodySource(t, devserverSrc, devserverSet, devserverAST, "run")
		for _, expected := range []string{
			"if devExitAfter > 0 {",
			"timer := time.NewTimer(devExitAfter)",
			"case <-timer.C:",
			"s.cleanupForRebuild()",
			"return nil",
		} {
			if !strings.Contains(runBody, expected) {
				t.Fatalf("expected run loop deterministic-exit behavior to include %q", expected)
			}
		}
	})
}
