package build_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildWatchEventsConformance(t *testing.T) {
	eventsPath := filepath.Join(repoRoot(t), "wave", "tooling", "events.go")
	eventsSrc, eventsSet, eventsAST := mustParseGoSourceFile(t, eventsPath)

	watcherPath := filepath.Join(repoRoot(t), "wave", "tooling", "watcher.go")
	watcherSrc, watcherSet, watcherAST := mustParseGoSourceFile(t, watcherPath)

	t.Run("BDC-EVT-001_BUILD-EVT-001_event_batches_deduplicate_work_by_path", func(t *testing.T) {
		processBody := mustFunctionBodySource(t, eventsSrc, eventsSet, eventsAST, "processEvents")
		for _, expected := range []string{
			"eventMap := make(map[string]fsnotify.Event)",
			"eventMap[evt.Name] = evt",
			"for _, evt := range eventMap {",
		} {
			if !strings.Contains(processBody, expected) {
				t.Fatalf("expected event dedupe contract to include %q", expected)
			}
		}
	})

	t.Run("BDC-EVT-002_BUILD-EVT-002_config_file_change_short_circuits_batch_with_config_restart", func(t *testing.T) {
		processBody := mustFunctionBodySource(t, eventsSrc, eventsSet, eventsAST, "processEvents")
		requireOrderedSubstrings(
			t,
			processBody,
			[]string{
				"if s.isConfigFile(evt.Name) && (evt.Has(fsnotify.Write) || evt.Has(fsnotify.Create)) {",
				"s.triggerConfigRestart()",
				"return",
			},
		)
	})

	t.Run("BDC-EVT-003_BUILD-EVT-003_non_empty_chmod_only_events_are_filtered_out", func(t *testing.T) {
		classifyBody := mustFunctionBodySource(t, eventsSrc, eventsSet, eventsAST, "classifyEventWithWatcherAndBuilder")
		if !strings.Contains(classifyBody, "result.chmodOnly = isNonEmptyChmodOnly(evt)") {
			t.Fatalf("expected classifier to mark chmod-only events")
		}

		processBody := mustFunctionBodySource(t, eventsSrc, eventsSet, eventsAST, "processEvents")
		if !strings.Contains(processBody, "if c.ignored || c.chmodOnly {") {
			t.Fatalf("expected processing to skip chmod-only events")
		}
	})

	t.Run("BDC-EVT-004_BUILD-EVT-004_event_classification_priority_and_treat_as_non_go_override_hold", func(t *testing.T) {
		classifyBody := mustFunctionBodySource(t, eventsSrc, eventsSet, eventsAST, "classifyEventWithWatcherAndBuilder")
		requireOrderedSubstrings(
			t,
			classifyBody,
			[]string{
				"if builder.IsCriticalCSSFile(evt.Name) {",
				"} else if builder.IsNormalCSSFile(evt.Name) {",
				"} else if filepath.Ext(evt.Name) == \".go\" {",
				"} else if watcher.IsPublicStaticFile(evt.Name) {",
				"} else if watcher.IsPrivateStaticFile(evt.Name) {",
			},
		)
		if !strings.Contains(classifyBody, "if result.fileType == fileTypeGo && result.watchedFile != nil && result.watchedFile.TreatAsNonGo {") {
			t.Fatalf("expected TreatAsNonGo override behavior")
		}
	})

	t.Run("BDC-EVT-005_BUILD-EVT-005_watched_file_merge_uses_union_trump_and_hook_order_semantics", func(t *testing.T) {
		mergeBody := mustFunctionBodySource(t, watcherSrc, watcherSet, watcherAST, "mergeWatchedFiles")
		for _, expected := range []string{
			"if wf.RecompileGoBinary {",
			"if wf.RestartApp {",
			"if !wf.TreatAsNonGo {",
			"if !wf.RunOnChangeOnly {",
			"if !wf.SkipRebuildingNotification {",
			"if wf.OnlyRunClientDefinedRevalidateFunc {",
			"allHooks = append(allHooks, wf.OnChangeHooks...)",
		} {
			if !strings.Contains(mergeBody, expected) {
				t.Fatalf("expected watched-file merge semantics to include %q", expected)
			}
		}
	})

	t.Run("BDC-EVT-006_BUILD-EVT-006_single_event_hook_phase_order_matches_contract", func(t *testing.T) {
		singleBody := mustFunctionBodySource(t, eventsSrc, eventsSet, eventsAST, "processSingleEvent")
		requireOrderedSubstrings(
			t,
			singleBody,
			[]string{
				"s.fireNoWaitHooks(ewh, watcher)",
				"preActions, err := s.runPreHooks(ewh, watcher)",
				"work.resolve(s.cfg.UsingVite())",
				"s.executeBuildPhase(work)",
				"actions, err := s.runConcurrentHooks(ewh, watcher)",
				"postActions, err := s.runPostHooks(ewh, watcher)",
				"if work.restartApp {",
				"s.executeBrowserPhase(work)",
			},
		)
	})

	t.Run("BDC-EVT-007_BUILD-EVT-007_batch_hard_reload_stops_app_once_and_runs_single_browser_phase", func(t *testing.T) {
		processBody := mustFunctionBodySource(t, eventsSrc, eventsSet, eventsAST, "processEvents")
		for _, expected := range []string{
			"if isBatch && batchNeedsAppStop {",
			"if err := s.stopApp(); err != nil {",
			"eventsWithHooks[i].hookCtx.AppStoppedForBatch = true",
		} {
			if !strings.Contains(processBody, expected) {
				t.Fatalf("expected batch stop-upfront behavior to include %q", expected)
			}
		}

		batchBody := mustFunctionBodySource(t, eventsSrc, eventsSet, eventsAST, "processBatchedEvents")
		if count := strings.Count(batchBody, "s.executeBrowserPhase(work)"); count != 1 {
			t.Fatalf("expected exactly one browser-phase execution for batched processing, got %d", count)
		}
	})

	t.Run("BDC-EVT-008_BUILD-EVT-008_browser_action_resolution_precedence_matches_workset_rules", func(t *testing.T) {
		behaviorBody := mustFunctionBodySource(t, eventsSrc, eventsSet, eventsAST, "determineBrowserBehavior")
		requireOrderedSubstrings(
			t,
			behaviorBody,
			[]string{
				"if w.restartApp {",
				"if w.preferRevalidate {",
				"if cssOnly {",
				"if w.processPublicFiles {",
				"if w.processPrivateFiles || cssWork {",
			},
		)
	})

	t.Run("BDC-EVT-009_BUILD-EVT-009_vite_invalidate_failure_falls_back_to_hard_reload_with_waits", func(t *testing.T) {
		browserBody := mustFunctionBodySource(t, eventsSrc, eventsSet, eventsAST, "executeBrowserPhase")
		for _, expected := range []string{
			"if work.invalidateVite {",
			"if err := s.callViteFilemapInvalidate(); err != nil {",
			"work.reloadBrowser = true",
			"work.waitForApp = true",
			"work.waitForVite = true",
		} {
			if !strings.Contains(browserBody, expected) {
				t.Fatalf("expected vite-invalidate fallback behavior to include %q", expected)
			}
		}
	})

	t.Run("BDC-EVT-010_BUILD-EVT-010_run_on_change_only_skips_standard_build_phase", func(t *testing.T) {
		addWorkBody := mustFunctionBodySource(t, eventsSrc, eventsSet, eventsAST, "addImplicitWork")
		if !strings.Contains(addWorkBody, "if wf != nil && wf.RunOnChangeOnly {") {
			t.Fatalf("expected addImplicitWork run-on-change-only early return")
		}

		singleBody := mustFunctionBodySource(t, eventsSrc, eventsSet, eventsAST, "processSingleEvent")
		if !strings.Contains(singleBody, "if ewh.runOnChangeOnly {") {
			t.Fatalf("expected single-event run-on-change-only build skip")
		}

		batchBody := mustFunctionBodySource(t, eventsSrc, eventsSet, eventsAST, "processBatchedEvents")
		if !strings.Contains(batchBody, "allRunOnChangeOnly := true") || !strings.Contains(batchBody, "if allRunOnChangeOnly {") {
			t.Fatalf("expected batched run-on-change-only short-circuit")
		}
	})
}
