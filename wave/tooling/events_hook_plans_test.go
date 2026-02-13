package tooling

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vormadev/vorma/wave"
)

func TestDeriveStageHooksAndRunOnChangePolicyForEvent(t *testing.T) {
	eventWithHooksForStage := eventWithHooks{
		hooks: &wave.SortedHooks{
			Pre: []wave.OnChangeHook{{Cmd: "pre"}},
			Concurrent: []wave.OnChangeHook{{
				Cmd: "concurrent",
			}},
			Post: []wave.OnChangeHook{{Cmd: "post"}},
			ConcurrentNoWait: []wave.OnChangeHook{{
				Cmd: "no-wait",
			}},
		},
	}

	testCases := []struct {
		name                             string
		stageType                        hookStageType
		expectedHookCount                int
		expectedRunOnChangeOnlyRuleUsage bool
		expectedFirstCommand             string
	}{
		{
			name:                             "pre stage",
			stageType:                        hookStageTypePre,
			expectedHookCount:                1,
			expectedRunOnChangeOnlyRuleUsage: false,
			expectedFirstCommand:             "pre",
		},
		{
			name:                             "concurrent stage",
			stageType:                        hookStageTypeConcurrent,
			expectedHookCount:                1,
			expectedRunOnChangeOnlyRuleUsage: true,
			expectedFirstCommand:             "concurrent",
		},
		{
			name:                             "post stage",
			stageType:                        hookStageTypePost,
			expectedHookCount:                1,
			expectedRunOnChangeOnlyRuleUsage: true,
			expectedFirstCommand:             "post",
		},
		{
			name:                             "concurrent-no-wait stage",
			stageType:                        hookStageTypeConcurrentNoWait,
			expectedHookCount:                1,
			expectedRunOnChangeOnlyRuleUsage: false,
			expectedFirstCommand:             "no-wait",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			stageHooks, shouldApplyRunOnChangeOnlyRules := deriveStageHooksAndRunOnChangePolicyForEvent(
				eventWithHooksForStage,
				testCase.stageType,
			)
			if len(stageHooks) != testCase.expectedHookCount {
				t.Fatalf("expected hook count %d, got %d", testCase.expectedHookCount, len(stageHooks))
			}
			if shouldApplyRunOnChangeOnlyRules != testCase.expectedRunOnChangeOnlyRuleUsage {
				t.Fatalf(
					"expected run-on-change-only rule usage %v, got %v",
					testCase.expectedRunOnChangeOnlyRuleUsage,
					shouldApplyRunOnChangeOnlyRules,
				)
			}
			if len(stageHooks) > 0 && stageHooks[0].Cmd != testCase.expectedFirstCommand {
				t.Fatalf(
					"expected first stage hook command %q, got %q",
					testCase.expectedFirstCommand,
					stageHooks[0].Cmd,
				)
			}
		})
	}

	nilStageHooks, shouldApplyRulesForNil := deriveStageHooksAndRunOnChangePolicyForEvent(
		eventWithHooks{},
		hookStageTypePre,
	)
	if nilStageHooks != nil {
		t.Fatalf("expected nil stage hooks for event without sorted hooks, got %#v", nilStageHooks)
	}
	if shouldApplyRulesForNil {
		t.Fatal("expected run-on-change-only rules to be disabled for event without sorted hooks")
	}
}

func TestDeriveHookExecutionPlansForEventStage_ConcurrentRunOnChangeOnlyFiltering(t *testing.T) {
	_, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	root := t.TempDir()
	changedPath := filepath.Join(root, "changed.css")
	if err := os.WriteFile(changedPath, []byte("body{}"), 0o644); err != nil {
		t.Fatalf("failed writing changed file: %v", err)
	}

	resolvedCommands := make([]string, 0)
	plans := deriveHookExecutionPlansForEventStage(
		watcher,
		eventWithHooks{
			classified:      classifiedEvent{event: waveEvent(changedPath)},
			runOnChangeOnly: true,
			hooks: &wave.SortedHooks{
				Concurrent: []wave.OnChangeHook{
					{Cmd: "echo command-only"},
					{
						Cmd: "echo callback-command",
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							return nil, nil
						},
					},
					{
						Exclude: []string{changedPath},
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							return nil, nil
						},
					},
				},
			},
		},
		hookStageTypeConcurrent,
		func(hook wave.OnChangeHook) string {
			resolvedCommands = append(resolvedCommands, hook.Cmd)
			return hook.Cmd
		},
	)

	if len(plans) != 1 {
		t.Fatalf("expected 1 planned hook for concurrent run-on-change-only stage, got %#v", plans)
	}
	if plans[0].callback == nil {
		t.Fatalf("expected planned hook callback to be retained, got %#v", plans[0])
	}
	if plans[0].command != "" {
		t.Fatalf("expected planned hook command to be stripped, got %#v", plans[0])
	}
	if len(resolvedCommands) != 1 {
		t.Fatalf("expected command resolver to run once for retained callback hook, got %d", len(resolvedCommands))
	}
	if resolvedCommands[0] != "" {
		t.Fatalf("expected command resolver to receive stripped command, got %q", resolvedCommands[0])
	}
}

func TestDeriveHookExecutionPlansForEventStage_PreStagePreservesCommands(t *testing.T) {
	_, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	root := t.TempDir()
	changedPath := filepath.Join(root, "changed.go")
	if err := os.WriteFile(changedPath, []byte("package main"), 0o644); err != nil {
		t.Fatalf("failed writing changed file: %v", err)
	}

	plans := deriveHookExecutionPlansForEventStage(
		watcher,
		eventWithHooks{
			classified:      classifiedEvent{event: waveEvent(changedPath)},
			runOnChangeOnly: true,
			hooks: &wave.SortedHooks{
				Pre: []wave.OnChangeHook{
					{Cmd: "echo first"},
					{
						Cmd: "echo second",
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							return nil, nil
						},
					},
				},
			},
		},
		hookStageTypePre,
		func(hook wave.OnChangeHook) string {
			return "resolved(" + hook.Cmd + ")"
		},
	)

	if len(plans) != 2 {
		t.Fatalf("expected 2 planned hooks for pre stage, got %#v", plans)
	}
	if plans[0].command != "resolved(echo first)" {
		t.Fatalf("expected first planned command to be resolved, got %#v", plans[0])
	}
	if plans[1].command != "resolved(echo second)" {
		t.Fatalf("expected second planned command to be resolved, got %#v", plans[1])
	}
	if plans[1].callback == nil {
		t.Fatalf("expected second planned hook callback to be retained, got %#v", plans[1])
	}
}
