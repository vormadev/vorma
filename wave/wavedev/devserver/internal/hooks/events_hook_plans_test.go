package hooks_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/wavedev/devserver/internal/eventpipeline"
	"github.com/vormadev/vorma/wave/wavedev/devserver/internal/hooks"
	"github.com/vormadev/vorma/wave/wavedev/internal/watch"
)

func TestDeriveStageHooksAndRunOnChangePolicyForEvent(t *testing.T) {
	eventWithHooksForStage := eventpipeline.EventWithHooks{
		Hooks: &wave.SortedHooks{
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
		Name                             string
		StageType                        hooks.HookStageType
		ExpectedHookCount                int
		ExpectedRunOnChangeOnlyRuleUsage bool
		ExpectedFirstCommand             string
	}{
		{
			Name:                             "pre stage",
			StageType:                        hooks.HookStageTypePre,
			ExpectedHookCount:                1,
			ExpectedRunOnChangeOnlyRuleUsage: false,
			ExpectedFirstCommand:             "pre",
		},
		{
			Name:                             "concurrent stage",
			StageType:                        hooks.HookStageTypeConcurrent,
			ExpectedHookCount:                1,
			ExpectedRunOnChangeOnlyRuleUsage: true,
			ExpectedFirstCommand:             "concurrent",
		},
		{
			Name:                             "post stage",
			StageType:                        hooks.HookStageTypePost,
			ExpectedHookCount:                1,
			ExpectedRunOnChangeOnlyRuleUsage: true,
			ExpectedFirstCommand:             "post",
		},
		{
			Name:                             "concurrent-no-wait stage",
			StageType:                        hooks.HookStageTypeConcurrentNoWait,
			ExpectedHookCount:                1,
			ExpectedRunOnChangeOnlyRuleUsage: false,
			ExpectedFirstCommand:             "no-wait",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			stageHooks, shouldApplyRunOnChangeOnlyRules := hooks.DeriveStageHooksAndRunOnChangePolicyForEvent(
				eventWithHooksForStage,
				testCase.StageType,
			)
			if len(stageHooks) != testCase.ExpectedHookCount {
				t.Fatalf(
					"expected hook count %d, got %d",
					testCase.ExpectedHookCount,
					len(stageHooks),
				)
			}
			if shouldApplyRunOnChangeOnlyRules != testCase.ExpectedRunOnChangeOnlyRuleUsage {
				t.Fatalf(
					"expected run-on-change-only rule usage %v, got %v",
					testCase.ExpectedRunOnChangeOnlyRuleUsage,
					shouldApplyRunOnChangeOnlyRules,
				)
			}
			if len(stageHooks) > 0 &&
				stageHooks[0].Cmd != testCase.ExpectedFirstCommand {
				t.Fatalf(
					"expected first stage hook command %q, got %q",
					testCase.ExpectedFirstCommand,
					stageHooks[0].Cmd,
				)
			}
		})
	}

	nilStageHooks, shouldApplyRulesForNil := hooks.DeriveStageHooksAndRunOnChangePolicyForEvent(
		eventpipeline.EventWithHooks{},
		hooks.HookStageTypePre,
	)
	if nilStageHooks != nil {
		t.Fatalf(
			"expected nil stage hooks for event without sorted hooks, got %#v",
			nilStageHooks,
		)
	}
	if shouldApplyRulesForNil {
		t.Fatal(
			"expected run-on-change-only rules to be disabled for event without sorted hooks",
		)
	}
}

func TestDeriveHookExecutionPlansForEventStage_ConcurrentRunOnChangeOnlyFiltering(
	t *testing.T,
) {
	watcherForTest := newWatcherForHookPlanTests(t)
	defer watcherForTest.Close()

	root := t.TempDir()
	changedPath := filepath.Join(root, "changed.css")
	if err := os.WriteFile(changedPath, []byte("body{}"), 0o644); err != nil {
		t.Fatalf("failed writing changed file: %v", err)
	}

	resolvedCommands := make([]string, 0)
	plans := hooks.DeriveHookExecutionPlansForEventStage(
		watcherForTest,
		eventpipeline.EventWithHooks{
			Classified: eventpipeline.ClassifiedEvent{
				Event: waveEventForHookPlanTests(changedPath),
			},
			RunOnChangeOnly: true,
			Hooks: &wave.SortedHooks{
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
		hooks.HookStageTypeConcurrent,
		func(hook wave.OnChangeHook) hooks.HookExecutionPlan {
			resolvedCommands = append(resolvedCommands, hook.Cmd)
			return hooks.DeriveHookExecutionPlanFromHook(
				hook,
				func(innerHook wave.OnChangeHook) string {
					return innerHook.Cmd
				},
			)
		},
	)

	if len(plans) != 1 {
		t.Fatalf(
			"expected 1 planned hook for concurrent run-on-change-only stage, got %#v",
			plans,
		)
	}
	if plans[0].Callback == nil {
		t.Fatalf(
			"expected planned hook callback to be retained, got %#v",
			plans[0],
		)
	}
	if plans[0].Command != "" {
		t.Fatalf(
			"expected planned hook command to be stripped, got %#v",
			plans[0],
		)
	}
	if len(resolvedCommands) != 1 {
		t.Fatalf(
			"expected command resolver to run once for retained callback hook, got %d",
			len(resolvedCommands),
		)
	}
	if resolvedCommands[0] != "" {
		t.Fatalf(
			"expected command resolver to receive stripped command, got %q",
			resolvedCommands[0],
		)
	}
}

func TestDeriveHookExecutionPlansForEventStage_PreStagePreservesCommands(
	t *testing.T,
) {
	watcherForTest := newWatcherForHookPlanTests(t)
	defer watcherForTest.Close()

	root := t.TempDir()
	changedPath := filepath.Join(root, "changed.go")
	if err := os.WriteFile(changedPath, []byte("package main"), 0o644); err != nil {
		t.Fatalf("failed writing changed file: %v", err)
	}

	plans := hooks.DeriveHookExecutionPlansForEventStage(
		watcherForTest,
		eventpipeline.EventWithHooks{
			Classified: eventpipeline.ClassifiedEvent{
				Event: waveEventForHookPlanTests(changedPath),
			},
			RunOnChangeOnly: true,
			Hooks: &wave.SortedHooks{
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
		hooks.HookStageTypePre,
		func(hook wave.OnChangeHook) hooks.HookExecutionPlan {
			return hooks.DeriveHookExecutionPlanFromHook(
				hook,
				func(innerHook wave.OnChangeHook) string {
					return "resolved(" + innerHook.Cmd + ")"
				},
			)
		},
	)

	if len(plans) != 2 {
		t.Fatalf("expected 2 planned hooks for pre stage, got %#v", plans)
	}
	if plans[0].Command != "resolved(echo first)" {
		t.Fatalf(
			"expected first planned command to be resolved, got %#v",
			plans[0],
		)
	}
	if plans[1].Command != "resolved(echo second)" {
		t.Fatalf(
			"expected second planned command to be resolved, got %#v",
			plans[1],
		)
	}
	if plans[1].Callback == nil {
		t.Fatalf(
			"expected second planned hook callback to be retained, got %#v",
			plans[1],
		)
	}
}

func newWatcherForHookPlanTests(t *testing.T) *watch.Watcher {
	t.Helper()

	root := t.TempDir()
	cfg := &wave.ParsedConfig{
		Core: &wave.CoreConfig{
			MainAppEntry: "cmd/app",
			DistDir:      filepath.Join(root, "dist"),
			StaticAssetDirs: staticAssetDirsForTests{
				Public:  filepath.Join(root, "static", "public"),
				Private: filepath.Join(root, "static", "private"),
			},
		},
		Watch: &wave.WatchConfig{
			WatchRoot: root,
		},
	}
	cfg.Dist.Root = cfg.Core.DistDir

	watcherForTest, watcherCreateError := watch.NewWatcher(
		cfg,
		nil,
	)
	if watcherCreateError != nil {
		t.Fatalf("watch.NewWatcher returned error: %v", watcherCreateError)
	}
	return watcherForTest
}

func waveEventForHookPlanTests(path string) fsnotify.Event {
	return fsnotify.Event{Name: path, Op: fsnotify.Write}
}
