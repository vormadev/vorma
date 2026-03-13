package evtparse_test

// import (
// 	"os"
// 	"path/filepath"
// 	"slices"
// 	"strings"
// 	"testing"

// 	"github.com/fsnotify/fsnotify"
// 	"github.com/vormadev/vorma/kit/strict"
// 	"github.com/vormadev/vorma/wave2/config"
// 	"github.com/vormadev/vorma/wave2/evtparse"
// )

// func TestRawBatchToWorkset_matrix(t *testing.T) {
// 	root_abs := t.TempDir()
// 	root_rel := rel_from_cwd(t, root_abs)

// 	mk_dir(t, filepath.Join(root_abs, "public"))
// 	mk_dir(t, filepath.Join(root_abs, "private"))
// 	touch_file(t, filepath.Join(root_abs, "main.go"))
// 	touch_file(t, filepath.Join(root_abs, "critical.css"))
// 	touch_file(t, filepath.Join(root_abs, "normal.css"))

// 	type matrix_case struct {
// 		name       string
// 		caller_ctx evtparse.CallerCtx
// 		mutate_cfg func(*config.Parsed)
// 		prepare_fs func(t *testing.T)
// 		raw_batch  func() []fsnotify.Event
// 		assert     func(t *testing.T, got evtparse.Workset)
// 	}

// 	test_cases := []matrix_case{
// 		{
// 			name: "empty-batch-short-circuits-noop",
// 			raw_batch: func() []fsnotify.Event {
// 				return nil
// 			},
// 			assert: func(t *testing.T, got evtparse.Workset) {
// 				assert_short_circuit(
// 					t,
// 					got,
// 					true,
// 					evtparse.ShortCircuitReasonNoOp,
// 				)
// 				if got.Watcher.PerformFullReset {
// 					t.Fatal("Watcher.PerformFullReset should be false")
// 				}
// 				if len(got.Watcher.WatchNewDirs) != 0 {
// 					t.Fatal("Watcher.WatchNewDirs should be empty")
// 				}
// 				if got.Builder.RunImplicitBuild {
// 					t.Fatal("Builder.RunImplicitBuild should be false")
// 				}
// 			},
// 		},
// 		{
// 			name: "config-change-has-highest-short-circuit-precedence",
// 			caller_ctx: evtparse.CallerCtx{
// 				WaitingForBuildRetry: true,
// 			},
// 			raw_batch: func() []fsnotify.Event {
// 				return []fsnotify.Event{
// 					raw_evt(path(root_rel, "wave.json"), fsnotify.Write),
// 					raw_evt(path(root_rel, "main.go"), fsnotify.Write),
// 				}
// 			},
// 			assert: func(t *testing.T, got evtparse.Workset) {
// 				assert_short_circuit(
// 					t,
// 					got,
// 					true,
// 					evtparse.ShortCircuitReasonConfigChange,
// 				)
// 			},
// 		},
// 		{
// 			name: "build-retry-wait-short-circuit-when-not-config-change",
// 			caller_ctx: evtparse.CallerCtx{
// 				WaitingForBuildRetry: true,
// 			},
// 			raw_batch: func() []fsnotify.Event {
// 				return []fsnotify.Event{
// 					raw_evt(path(root_rel, "main.go"), fsnotify.Write),
// 				}
// 			},
// 			assert: func(t *testing.T, got evtparse.Workset) {
// 				assert_short_circuit(
// 					t,
// 					got,
// 					true,
// 					evtparse.ShortCircuitReasonBuildRetryWait,
// 				)
// 			},
// 		},
// 		{
// 			name: "excluded-create-dir-does-not-populate-watch-new-dirs",
// 			mutate_cfg: func(cfg *config.Parsed) {
// 				cfg.Watch.Exclude = []strict.CWDRelPath{
// 					glob(root_rel, "ignored", "**"),
// 				}
// 			},
// 			prepare_fs: func(t *testing.T) {
// 				mk_dir(t, filepath.Join(root_abs, "ignored", "newdir"))
// 			},
// 			raw_batch: func() []fsnotify.Event {
// 				return []fsnotify.Event{
// 					raw_evt(
// 						path(root_rel, "ignored", "newdir"),
// 						fsnotify.Create,
// 					),
// 				}
// 			},
// 			assert: func(t *testing.T, got evtparse.Workset) {
// 				assert_short_circuit(
// 					t,
// 					got,
// 					true,
// 					evtparse.ShortCircuitReasonNoOp,
// 				)
// 				if len(got.Watcher.WatchNewDirs) != 0 {
// 					t.Fatal("Watcher.WatchNewDirs should be empty")
// 				}
// 			},
// 		},
// 		{
// 			name: "watch-new-dirs-is-deduped-and-sorted",
// 			prepare_fs: func(t *testing.T) {
// 				mk_dir(t, filepath.Join(root_abs, "content", "b"))
// 				mk_dir(t, filepath.Join(root_abs, "content", "a"))
// 			},
// 			raw_batch: func() []fsnotify.Event {
// 				return []fsnotify.Event{
// 					raw_evt(path(root_rel, "content", "b"), fsnotify.Create),
// 					raw_evt(path(root_rel, "content", "a"), fsnotify.Create),
// 					raw_evt(path(root_rel, "content", "a"), fsnotify.Create),
// 				}
// 			},
// 			assert: func(t *testing.T, got evtparse.Workset) {
// 				assert_short_circuit(
// 					t,
// 					got,
// 					false,
// 					evtparse.ShortCircuitNotApplicable,
// 				)
// 				want := []strict.CWDRelPath{
// 					path(root_rel, "content", "a"),
// 					path(root_rel, "content", "b"),
// 				}
// 				if !slices.Equal(got.Watcher.WatchNewDirs, want) {
// 					t.Fatalf(
// 						"Watcher.WatchNewDirs = %#v, want %#v",
// 						got.Watcher.WatchNewDirs,
// 						want,
// 					)
// 				}
// 			},
// 		},
// 		{
// 			name: "delete-event-triggers-watcher-full-reset",
// 			raw_batch: func() []fsnotify.Event {
// 				return []fsnotify.Event{
// 					raw_evt(
// 						path(root_rel, "content", "deleted.txt"),
// 						fsnotify.Remove,
// 					),
// 				}
// 			},
// 			assert: func(t *testing.T, got evtparse.Workset) {
// 				assert_short_circuit(
// 					t,
// 					got,
// 					false,
// 					evtparse.ShortCircuitNotApplicable,
// 				)
// 				if !got.Watcher.PerformFullReset {
// 					t.Fatal("Watcher.PerformFullReset should be true")
// 				}
// 			},
// 		},
// 		{
// 			name: "hooks-dedupe-and-preserve-first-seen-order-across-includes-and-events",
// 			mutate_cfg: func(cfg *config.Parsed) {
// 				cfg.Watch.Include = []config.ParsedWatchIncludeEntry{
// 					{
// 						Pattern: glob(
// 							root_rel,
// 							"content",
// 							"**",
// 							"*.txt",
// 						),
// 						RunOnChangeOnly: true,
// 						OnChangeHooks: []config.ParsedOnChangeHook{
// 							{
// 								Cmd:    string("pre-a"),
// 								Timing: config.OnChangeHookTimingPre,
// 							},
// 							{
// 								Cmd:    string("pre-a"),
// 								Timing: config.OnChangeHookTimingPre,
// 							},
// 							{
// 								Cmd:    string("conc-a"),
// 								Timing: config.OnChangeHookTimingConcurrent,
// 							},
// 							{
// 								Cmd:    string("post-a"),
// 								Timing: config.OnChangeHookTimingPost,
// 							},
// 						},
// 					},
// 					{
// 						Pattern: glob(
// 							root_rel,
// 							"content",
// 							"**",
// 							"*.txt",
// 						),
// 						RunOnChangeOnly: true,
// 						OnChangeHooks: []config.ParsedOnChangeHook{
// 							{
// 								Cmd:    string("pre-b"),
// 								Timing: config.OnChangeHookTimingPre,
// 							},
// 							{
// 								Cmd:    string("conc-a"),
// 								Timing: config.OnChangeHookTimingConcurrent,
// 							},
// 							{
// 								Cmd:    string("no-wait-a"),
// 								Timing: config.OnChangeHookTimingConcurrentNoWait,
// 							},
// 						},
// 					},
// 				}
// 			},
// 			raw_batch: func() []fsnotify.Event {
// 				return []fsnotify.Event{
// 					raw_evt(
// 						path(root_rel, "content", "one.txt"),
// 						fsnotify.Write,
// 					),
// 					raw_evt(
// 						path(root_rel, "content", "two.txt"),
// 						fsnotify.Write,
// 					),
// 				}
// 			},
// 			assert: func(t *testing.T, got evtparse.Workset) {
// 				assert_short_circuit(
// 					t,
// 					got,
// 					false,
// 					evtparse.ShortCircuitNotApplicable,
// 				)
// 				if got.Builder.RunImplicitBuild {
// 					t.Fatal("Builder.RunImplicitBuild should be false")
// 				}
// 				if !slices.Equal(
// 					got.Builder.HookCmds.Pre,
// 					[]string{"pre-a", "pre-b"},
// 				) {
// 					t.Fatalf(
// 						"Builder.HookCmds.Pre = %#v",
// 						got.Builder.HookCmds.Pre,
// 					)
// 				}
// 				if !slices.Equal(
// 					got.Builder.HookCmds.Concurrent,
// 					[]string{"conc-a"},
// 				) {
// 					t.Fatalf(
// 						"Builder.HookCmds.Concurrent = %#v",
// 						got.Builder.HookCmds.Concurrent,
// 					)
// 				}
// 				if !slices.Equal(
// 					got.Builder.HookCmds.ConcurrentNoWait,
// 					[]string{"no-wait-a"},
// 				) {
// 					t.Fatalf(
// 						"Builder.HookCmds.ConcurrentNoWait = %#v",
// 						got.Builder.HookCmds.ConcurrentNoWait,
// 					)
// 				}
// 				if !slices.Equal(
// 					got.Builder.HookCmds.Post,
// 					[]string{"post-a"},
// 				) {
// 					t.Fatalf(
// 						"Builder.HookCmds.Post = %#v",
// 						got.Builder.HookCmds.Post,
// 					)
// 				}
// 			},
// 		},
// 		{
// 			name: "same-pattern-conflict-run-on-change-only-false-dominates",
// 			mutate_cfg: func(cfg *config.Parsed) {
// 				cfg.Watch.Include = []config.ParsedWatchIncludeEntry{
// 					{
// 						Pattern:         glob(root_rel, "docs", "**", "*.md"),
// 						RunOnChangeOnly: true,
// 					},
// 					{
// 						Pattern:         glob(root_rel, "docs", "**", "*.md"),
// 						RunOnChangeOnly: false,
// 					},
// 				}
// 			},
// 			raw_batch: func() []fsnotify.Event {
// 				return []fsnotify.Event{
// 					raw_evt(path(root_rel, "docs", "post.md"), fsnotify.Write),
// 				}
// 			},
// 			assert: func(t *testing.T, got evtparse.Workset) {
// 				assert_short_circuit(
// 					t,
// 					got,
// 					false,
// 					evtparse.ShortCircuitNotApplicable,
// 				)
// 				if !got.Builder.RunImplicitBuild {
// 					t.Fatal("Builder.RunImplicitBuild should be true")
// 				}
// 			},
// 		},
// 		{
// 			name: "same-pattern-conflict-run-on-change-only-true-when-all-true",
// 			mutate_cfg: func(cfg *config.Parsed) {
// 				cfg.Watch.Include = []config.ParsedWatchIncludeEntry{
// 					{
// 						Pattern:         glob(root_rel, "docs", "**", "*.md"),
// 						RunOnChangeOnly: true,
// 						OnChangeHooks: []config.ParsedOnChangeHook{
// 							{
// 								Cmd:    string("hook-a"),
// 								Timing: config.OnChangeHookTimingPre,
// 							},
// 						},
// 					},
// 					{
// 						Pattern:         glob(root_rel, "docs", "**", "*.md"),
// 						RunOnChangeOnly: true,
// 					},
// 				}
// 			},
// 			raw_batch: func() []fsnotify.Event {
// 				return []fsnotify.Event{
// 					raw_evt(path(root_rel, "docs", "post.md"), fsnotify.Write),
// 				}
// 			},
// 			assert: func(t *testing.T, got evtparse.Workset) {
// 				assert_short_circuit(
// 					t,
// 					got,
// 					false,
// 					evtparse.ShortCircuitNotApplicable,
// 				)
// 				if got.Builder.RunImplicitBuild {
// 					t.Fatal("Builder.RunImplicitBuild should be false")
// 				}
// 				if !slices.Equal(
// 					got.Builder.HookCmds.Pre,
// 					[]string{"hook-a"},
// 				) {
// 					t.Fatalf(
// 						"Builder.HookCmds.Pre = %#v, want %#v",
// 						got.Builder.HookCmds.Pre,
// 						[]string{"hook-a"},
// 					)
// 				}
// 			},
// 		},
// 		{
// 			name: "same-pattern-conflict-treat-as-non-go-false-dominates",
// 			mutate_cfg: func(cfg *config.Parsed) {
// 				cfg.Watch.Include = []config.ParsedWatchIncludeEntry{
// 					{
// 						Pattern:      glob(root_rel, "src", "**", "*.go"),
// 						TreatAsNonGo: true,
// 					},
// 					{
// 						Pattern:      glob(root_rel, "src", "**", "*.go"),
// 						TreatAsNonGo: false,
// 					},
// 				}
// 			},
// 			raw_batch: func() []fsnotify.Event {
// 				return []fsnotify.Event{
// 					raw_evt(path(root_rel, "src", "app.go"), fsnotify.Write),
// 				}
// 			},
// 			assert: func(t *testing.T, got evtparse.Workset) {
// 				assert_short_circuit(
// 					t,
// 					got,
// 					false,
// 					evtparse.ShortCircuitNotApplicable,
// 				)
// 				if !got.Builder.CompileGoBinary {
// 					t.Fatal("Builder.CompileGoBinary should be true")
// 				}
// 				if !got.Builder.RestartAppServer {
// 					t.Fatal("Builder.RestartAppServer should be true")
// 				}
// 			},
// 		},
// 		{
// 			name: "same-pattern-conflict-treat-as-non-go-true-when-all-true",
// 			mutate_cfg: func(cfg *config.Parsed) {
// 				cfg.Watch.Include = []config.ParsedWatchIncludeEntry{
// 					{
// 						Pattern:      glob(root_rel, "src", "**", "*.go"),
// 						TreatAsNonGo: true,
// 					},
// 					{
// 						Pattern:      glob(root_rel, "src", "**", "*.go"),
// 						TreatAsNonGo: true,
// 					},
// 				}
// 			},
// 			raw_batch: func() []fsnotify.Event {
// 				return []fsnotify.Event{
// 					raw_evt(path(root_rel, "src", "app.go"), fsnotify.Write),
// 				}
// 			},
// 			assert: func(t *testing.T, got evtparse.Workset) {
// 				assert_short_circuit(
// 					t,
// 					got,
// 					false,
// 					evtparse.ShortCircuitNotApplicable,
// 				)
// 				if got.Builder.CompileGoBinary {
// 					t.Fatal("Builder.CompileGoBinary should be false")
// 				}
// 				if got.Builder.RestartAppServer {
// 					t.Fatal("Builder.RestartAppServer should be false")
// 				}
// 				if !got.Builder.RunImplicitBuild {
// 					t.Fatal("Builder.RunImplicitBuild should be true")
// 				}
// 			},
// 		},
// 		{
// 			name: "same-pattern-conflict-revalidate-and-recompile-unions-conservatively",
// 			mutate_cfg: func(cfg *config.Parsed) {
// 				cfg.Watch.Include = []config.ParsedWatchIncludeEntry{
// 					{
// 						Pattern: glob(
// 							root_rel,
// 							"content",
// 							"**",
// 							"*.md",
// 						),
// 						OnlyRunClientDefinedRevalidateFunc: true,
// 					},
// 					{
// 						Pattern: glob(
// 							root_rel,
// 							"content",
// 							"**",
// 							"*.md",
// 						),
// 						RecompileGoBinary: true,
// 					},
// 				}
// 			},
// 			raw_batch: func() []fsnotify.Event {
// 				return []fsnotify.Event{
// 					raw_evt(
// 						path(root_rel, "content", "post.md"),
// 						fsnotify.Write,
// 					),
// 				}
// 			},
// 			assert: func(t *testing.T, got evtparse.Workset) {
// 				assert_short_circuit(
// 					t,
// 					got,
// 					false,
// 					evtparse.ShortCircuitNotApplicable,
// 				)
// 				if !got.Builder.PreferFrontendRevalidate {
// 					t.Fatal("Builder.PreferFrontendRevalidate should be true")
// 				}
// 				if !got.Builder.CompileGoBinary {
// 					t.Fatal("Builder.CompileGoBinary should be true")
// 				}
// 				if !got.Builder.RestartAppServer {
// 					t.Fatal("Builder.RestartAppServer should be true")
// 				}
// 			},
// 		},
// 		{
// 			name: "same-pattern-conflict-revalidate-and-restart-unions-conservatively",
// 			mutate_cfg: func(cfg *config.Parsed) {
// 				cfg.Watch.Include = []config.ParsedWatchIncludeEntry{
// 					{
// 						Pattern: glob(
// 							root_rel,
// 							"content",
// 							"**",
// 							"*.md",
// 						),
// 						OnlyRunClientDefinedRevalidateFunc: true,
// 					},
// 					{
// 						Pattern:    glob(root_rel, "content", "**", "*.md"),
// 						RestartApp: true,
// 					},
// 				}
// 			},
// 			raw_batch: func() []fsnotify.Event {
// 				return []fsnotify.Event{
// 					raw_evt(
// 						path(root_rel, "content", "post.md"),
// 						fsnotify.Write,
// 					),
// 				}
// 			},
// 			assert: func(t *testing.T, got evtparse.Workset) {
// 				assert_short_circuit(
// 					t,
// 					got,
// 					false,
// 					evtparse.ShortCircuitNotApplicable,
// 				)
// 				if !got.Builder.PreferFrontendRevalidate {
// 					t.Fatal("Builder.PreferFrontendRevalidate should be true")
// 				}
// 				if got.Builder.CompileGoBinary {
// 					t.Fatal("Builder.CompileGoBinary should be false")
// 				}
// 				if !got.Builder.RestartAppServer {
// 					t.Fatal("Builder.RestartAppServer should be true")
// 				}
// 			},
// 		},
// 		{
// 			name: "same-pattern-conflict-skip-overlay-false-dominates",
// 			mutate_cfg: func(cfg *config.Parsed) {
// 				cfg.Watch.Include = []config.ParsedWatchIncludeEntry{
// 					{
// 						Pattern: glob(
// 							root_rel,
// 							"content",
// 							"**",
// 							"*.txt",
// 						),
// 						RunOnChangeOnly:            true,
// 						SkipRebuildingNotification: true,
// 						OnChangeHooks: []config.ParsedOnChangeHook{
// 							{
// 								Cmd:    string("hook-a"),
// 								Timing: config.OnChangeHookTimingPre,
// 							},
// 						},
// 					},
// 					{
// 						Pattern: glob(
// 							root_rel,
// 							"content",
// 							"**",
// 							"*.txt",
// 						),
// 						RunOnChangeOnly:            true,
// 						SkipRebuildingNotification: false,
// 						OnChangeHooks: []config.ParsedOnChangeHook{
// 							{
// 								Cmd:    string("hook-b"),
// 								Timing: config.OnChangeHookTimingPre,
// 							},
// 						},
// 					},
// 				}
// 			},
// 			raw_batch: func() []fsnotify.Event {
// 				return []fsnotify.Event{
// 					raw_evt(
// 						path(root_rel, "content", "item.txt"),
// 						fsnotify.Write,
// 					),
// 				}
// 			},
// 			assert: func(t *testing.T, got evtparse.Workset) {
// 				assert_short_circuit(
// 					t,
// 					got,
// 					false,
// 					evtparse.ShortCircuitNotApplicable,
// 				)
// 				if got.Builder.SkipFrontendRebuildingOverlay {
// 					t.Fatal(
// 						"Builder.SkipFrontendRebuildingOverlay should be false",
// 					)
// 				}
// 			},
// 		},
// 		{
// 			name: "same-pattern-conflict-skip-overlay-true-when-all-true",
// 			mutate_cfg: func(cfg *config.Parsed) {
// 				cfg.Watch.Include = []config.ParsedWatchIncludeEntry{
// 					{
// 						Pattern: glob(
// 							root_rel,
// 							"content",
// 							"**",
// 							"*.txt",
// 						),
// 						RunOnChangeOnly:            true,
// 						SkipRebuildingNotification: true,
// 						OnChangeHooks: []config.ParsedOnChangeHook{
// 							{
// 								Cmd:    string("hook-a"),
// 								Timing: config.OnChangeHookTimingPre,
// 							},
// 						},
// 					},
// 					{
// 						Pattern: glob(
// 							root_rel,
// 							"content",
// 							"**",
// 							"*.txt",
// 						),
// 						RunOnChangeOnly:            true,
// 						SkipRebuildingNotification: true,
// 						OnChangeHooks: []config.ParsedOnChangeHook{
// 							{
// 								Cmd:    string("hook-b"),
// 								Timing: config.OnChangeHookTimingPre,
// 							},
// 						},
// 					},
// 				}
// 			},
// 			raw_batch: func() []fsnotify.Event {
// 				return []fsnotify.Event{
// 					raw_evt(
// 						path(root_rel, "content", "item.txt"),
// 						fsnotify.Write,
// 					),
// 				}
// 			},
// 			assert: func(t *testing.T, got evtparse.Workset) {
// 				assert_short_circuit(
// 					t,
// 					got,
// 					false,
// 					evtparse.ShortCircuitNotApplicable,
// 				)
// 				if !got.Builder.SkipFrontendRebuildingOverlay {
// 					t.Fatal(
// 						"Builder.SkipFrontendRebuildingOverlay should be true",
// 					)
// 				}
// 			},
// 		},
// 		{
// 			name: "same-pattern-overlapping-hooks-keep-cross-lane-cmds",
// 			mutate_cfg: func(cfg *config.Parsed) {
// 				cfg.Watch.Include = []config.ParsedWatchIncludeEntry{
// 					{
// 						Pattern: glob(
// 							root_rel,
// 							"content",
// 							"**",
// 							"*.txt",
// 						),
// 						RunOnChangeOnly: true,
// 						OnChangeHooks: []config.ParsedOnChangeHook{
// 							{
// 								Cmd:    string("a"),
// 								Timing: config.OnChangeHookTimingPre,
// 							},
// 							{
// 								Cmd:    string("b"),
// 								Timing: config.OnChangeHookTimingConcurrent,
// 							},
// 						},
// 					},
// 					{
// 						Pattern: glob(
// 							root_rel,
// 							"content",
// 							"**",
// 							"*.txt",
// 						),
// 						RunOnChangeOnly: true,
// 						OnChangeHooks: []config.ParsedOnChangeHook{
// 							{
// 								Cmd:    string("a"),
// 								Timing: config.OnChangeHookTimingPre,
// 							},
// 							{
// 								Cmd:    string("b"),
// 								Timing: config.OnChangeHookTimingPost,
// 							},
// 							{
// 								Cmd:    string("c"),
// 								Timing: config.OnChangeHookTimingConcurrentNoWait,
// 							},
// 							{
// 								Cmd:    string("d"),
// 								Timing: config.OnChangeHookTimingPost,
// 							},
// 						},
// 					},
// 				}
// 			},
// 			raw_batch: func() []fsnotify.Event {
// 				return []fsnotify.Event{
// 					raw_evt(
// 						path(root_rel, "content", "item.txt"),
// 						fsnotify.Write,
// 					),
// 				}
// 			},
// 			assert: func(t *testing.T, got evtparse.Workset) {
// 				assert_short_circuit(
// 					t,
// 					got,
// 					false,
// 					evtparse.ShortCircuitNotApplicable,
// 				)
// 				if !slices.Equal(
// 					got.Builder.HookCmds.Pre,
// 					[]string{"a"},
// 				) {
// 					t.Fatalf(
// 						"Builder.HookCmds.Pre = %#v, want %#v",
// 						got.Builder.HookCmds.Pre,
// 						[]string{"a"},
// 					)
// 				}
// 				if !slices.Equal(
// 					got.Builder.HookCmds.Concurrent,
// 					[]string{"b"},
// 				) {
// 					t.Fatalf(
// 						"Builder.HookCmds.Concurrent = %#v, want %#v",
// 						got.Builder.HookCmds.Concurrent,
// 						[]string{"b"},
// 					)
// 				}
// 				if !slices.Equal(
// 					got.Builder.HookCmds.ConcurrentNoWait,
// 					[]string{"c"},
// 				) {
// 					t.Fatalf(
// 						"Builder.HookCmds.ConcurrentNoWait = %#v, want %#v",
// 						got.Builder.HookCmds.ConcurrentNoWait,
// 						[]string{"c"},
// 					)
// 				}
// 				if !slices.Equal(
// 					got.Builder.HookCmds.Post,
// 					[]string{"b", "d"},
// 				) {
// 					t.Fatalf(
// 						"Builder.HookCmds.Post = %#v, want %#v",
// 						got.Builder.HookCmds.Post,
// 						[]string{"b", "d"},
// 					)
// 				}
// 			},
// 		},
// 		{
// 			name: "mixed-event-conflicts-reduce-to-conservative-union-workset",
// 			mutate_cfg: func(cfg *config.Parsed) {
// 				cfg.Watch.Include = []config.ParsedWatchIncludeEntry{
// 					{
// 						Pattern:      glob(root_rel, "src", "**", "*.go"),
// 						TreatAsNonGo: true,
// 					},
// 					{
// 						Pattern:      glob(root_rel, "src", "**", "*.go"),
// 						TreatAsNonGo: false,
// 					},
// 					{
// 						Pattern: glob(
// 							root_rel,
// 							"content",
// 							"**",
// 							"*.md",
// 						),
// 						OnlyRunClientDefinedRevalidateFunc: true,
// 						OnChangeHooks: []config.ParsedOnChangeHook{
// 							{
// 								Cmd:    string("md-hook"),
// 								Timing: config.OnChangeHookTimingPre,
// 							},
// 						},
// 					},
// 					{
// 						Pattern:    glob(root_rel, "content", "**", "*.md"),
// 						RestartApp: true,
// 					},
// 				}
// 			},
// 			raw_batch: func() []fsnotify.Event {
// 				return []fsnotify.Event{
// 					raw_evt(path(root_rel, "src", "app.go"), fsnotify.Write),
// 					raw_evt(
// 						path(root_rel, "content", "post.md"),
// 						fsnotify.Write,
// 					),
// 					raw_evt(
// 						path(root_rel, "public", "logo.svg"),
// 						fsnotify.Write,
// 					),
// 				}
// 			},
// 			assert: func(t *testing.T, got evtparse.Workset) {
// 				assert_short_circuit(
// 					t,
// 					got,
// 					false,
// 					evtparse.ShortCircuitNotApplicable,
// 				)
// 				if !got.Builder.RunImplicitBuild {
// 					t.Fatal("Builder.RunImplicitBuild should be true")
// 				}
// 				if !got.Builder.CompileGoBinary {
// 					t.Fatal("Builder.CompileGoBinary should be true")
// 				}
// 				if !got.Builder.RestartAppServer {
// 					t.Fatal("Builder.RestartAppServer should be true")
// 				}
// 				if !got.Builder.PreferFrontendRevalidate {
// 					t.Fatal("Builder.PreferFrontendRevalidate should be true")
// 				}
// 				if !got.Builder.HandlePublicStatic {
// 					t.Fatal("Builder.HandlePublicStatic should be true")
// 				}
// 				if !slices.Equal(
// 					got.Builder.HookCmds.Pre,
// 					[]string{"md-hook"},
// 				) {
// 					t.Fatalf(
// 						"Builder.HookCmds.Pre = %#v, want %#v",
// 						got.Builder.HookCmds.Pre,
// 						[]string{"md-hook"},
// 					)
// 				}
// 			},
// 		},
// 		{
// 			name: "run-on-change-only-include-executes-hooks-without-implicit-build",
// 			mutate_cfg: func(cfg *config.Parsed) {
// 				cfg.Watch.Include = []config.ParsedWatchIncludeEntry{
// 					{
// 						Pattern:         glob(root_rel, "docs", "**", "*.md"),
// 						RunOnChangeOnly: true,
// 						OnChangeHooks: []config.ParsedOnChangeHook{
// 							{
// 								Cmd:    string("hook-pre"),
// 								Timing: config.OnChangeHookTimingPre,
// 							},
// 						},
// 					},
// 				}
// 			},
// 			raw_batch: func() []fsnotify.Event {
// 				return []fsnotify.Event{
// 					raw_evt(path(root_rel, "docs", "post.md"), fsnotify.Write),
// 				}
// 			},
// 			assert: func(t *testing.T, got evtparse.Workset) {
// 				assert_short_circuit(
// 					t,
// 					got,
// 					false,
// 					evtparse.ShortCircuitNotApplicable,
// 				)
// 				if got.Builder.RunImplicitBuild {
// 					t.Fatal("Builder.RunImplicitBuild should be false")
// 				}
// 				if got.Builder.CompileGoBinary {
// 					t.Fatal("Builder.CompileGoBinary should be false")
// 				}
// 				if got.Builder.RestartAppServer {
// 					t.Fatal("Builder.RestartAppServer should be false")
// 				}
// 				if !slices.Equal(
// 					got.Builder.HookCmds.Pre,
// 					[]string{"hook-pre"},
// 				) {
// 					t.Fatalf(
// 						"Builder.HookCmds.Pre = %#v, want %#v",
// 						got.Builder.HookCmds.Pre,
// 						[]string{"hook-pre"},
// 					)
// 				}
// 			},
// 		},
// 		{
// 			name: "go-event-without-include-runs-implicit-build-and-restart",
// 			raw_batch: func() []fsnotify.Event {
// 				return []fsnotify.Event{
// 					raw_evt(path(root_rel, "src", "app.go"), fsnotify.Write),
// 				}
// 			},
// 			assert: func(t *testing.T, got evtparse.Workset) {
// 				assert_short_circuit(
// 					t,
// 					got,
// 					false,
// 					evtparse.ShortCircuitNotApplicable,
// 				)
// 				if !got.Builder.RunImplicitBuild {
// 					t.Fatal("Builder.RunImplicitBuild should be true")
// 				}
// 				if !got.Builder.CompileGoBinary {
// 					t.Fatal("Builder.CompileGoBinary should be true")
// 				}
// 				if !got.Builder.RestartAppServer {
// 					t.Fatal("Builder.RestartAppServer should be true")
// 				}
// 			},
// 		},
// 		{
// 			name: "treat-as-non-go-suppresses-go-classification",
// 			mutate_cfg: func(cfg *config.Parsed) {
// 				cfg.Watch.Include = []config.ParsedWatchIncludeEntry{
// 					{
// 						Pattern:      glob(root_rel, "src", "**", "*.go"),
// 						TreatAsNonGo: true,
// 					},
// 				}
// 			},
// 			raw_batch: func() []fsnotify.Event {
// 				return []fsnotify.Event{
// 					raw_evt(path(root_rel, "src", "app.go"), fsnotify.Write),
// 				}
// 			},
// 			assert: func(t *testing.T, got evtparse.Workset) {
// 				assert_short_circuit(
// 					t,
// 					got,
// 					false,
// 					evtparse.ShortCircuitNotApplicable,
// 				)
// 				if !got.Builder.RunImplicitBuild {
// 					t.Fatal("Builder.RunImplicitBuild should be true")
// 				}
// 				if got.Builder.CompileGoBinary {
// 					t.Fatal("Builder.CompileGoBinary should be false")
// 				}
// 				if got.Builder.RestartAppServer {
// 					t.Fatal("Builder.RestartAppServer should be false")
// 				}
// 			},
// 		},
// 		{
// 			name: "critical-css-can-trigger-restart-via-include-restart",
// 			mutate_cfg: func(cfg *config.Parsed) {
// 				cfg.Watch.Include = []config.ParsedWatchIncludeEntry{
// 					{
// 						Pattern:    path(root_rel, "critical.css"),
// 						RestartApp: true,
// 					},
// 				}
// 			},
// 			raw_batch: func() []fsnotify.Event {
// 				return []fsnotify.Event{
// 					raw_evt(path(root_rel, "critical.css"), fsnotify.Write),
// 				}
// 			},
// 			assert: func(t *testing.T, got evtparse.Workset) {
// 				assert_short_circuit(
// 					t,
// 					got,
// 					false,
// 					evtparse.ShortCircuitNotApplicable,
// 				)
// 				if !got.Builder.HandleCriticalCSS {
// 					t.Fatal("Builder.HandleCriticalCSS should be true")
// 				}
// 				if !got.Builder.RestartAppServer {
// 					t.Fatal("Builder.RestartAppServer should be true")
// 				}
// 			},
// 		},
// 		{
// 			name: "public-and-private-static-events-toggle-both-handlers",
// 			raw_batch: func() []fsnotify.Event {
// 				return []fsnotify.Event{
// 					raw_evt(
// 						path(root_rel, "public", "logo.svg"),
// 						fsnotify.Write,
// 					),
// 					raw_evt(
// 						path(root_rel, "private", "doc.txt"),
// 						fsnotify.Write,
// 					),
// 				}
// 			},
// 			assert: func(t *testing.T, got evtparse.Workset) {
// 				assert_short_circuit(
// 					t,
// 					got,
// 					false,
// 					evtparse.ShortCircuitNotApplicable,
// 				)
// 				if !got.Builder.HandlePublicStatic {
// 					t.Fatal("Builder.HandlePublicStatic should be true")
// 				}
// 				if !got.Builder.HandlePrivateStatic {
// 					t.Fatal("Builder.HandlePrivateStatic should be true")
// 				}
// 			},
// 		},
// 		{
// 			name: "skip-frontend-overlay-uses-and-semantics-across-processed-events",
// 			mutate_cfg: func(cfg *config.Parsed) {
// 				cfg.Watch.Include = []config.ParsedWatchIncludeEntry{
// 					{
// 						Pattern: path(
// 							root_rel,
// 							"content",
// 							"a.txt",
// 						),
// 						RunOnChangeOnly:            true,
// 						SkipRebuildingNotification: true,
// 						OnChangeHooks: []config.ParsedOnChangeHook{
// 							{
// 								Cmd:    string("a"),
// 								Timing: config.OnChangeHookTimingPre,
// 							},
// 						},
// 					},
// 					{
// 						Pattern:         path(root_rel, "content", "b.txt"),
// 						RunOnChangeOnly: true,
// 						OnChangeHooks: []config.ParsedOnChangeHook{
// 							{
// 								Cmd:    string("b"),
// 								Timing: config.OnChangeHookTimingPre,
// 							},
// 						},
// 					},
// 				}
// 			},
// 			raw_batch: func() []fsnotify.Event {
// 				return []fsnotify.Event{
// 					raw_evt(path(root_rel, "content", "a.txt"), fsnotify.Write),
// 					raw_evt(path(root_rel, "content", "b.txt"), fsnotify.Write),
// 				}
// 			},
// 			assert: func(t *testing.T, got evtparse.Workset) {
// 				assert_short_circuit(
// 					t,
// 					got,
// 					false,
// 					evtparse.ShortCircuitNotApplicable,
// 				)
// 				if got.Builder.SkipFrontendRebuildingOverlay {
// 					t.Fatal(
// 						"Builder.SkipFrontendRebuildingOverlay should be false",
// 					)
// 				}
// 			},
// 		},
// 		{
// 			name: "prefer-frontend-revalidate-is-derived-from-include",
// 			mutate_cfg: func(cfg *config.Parsed) {
// 				cfg.Watch.Include = []config.ParsedWatchIncludeEntry{
// 					{
// 						Pattern: path(
// 							root_rel,
// 							"content",
// 							"revalidate.md",
// 						),
// 						OnlyRunClientDefinedRevalidateFunc: true,
// 					},
// 				}
// 			},
// 			raw_batch: func() []fsnotify.Event {
// 				return []fsnotify.Event{
// 					raw_evt(
// 						path(root_rel, "content", "revalidate.md"),
// 						fsnotify.Write,
// 					),
// 				}
// 			},
// 			assert: func(t *testing.T, got evtparse.Workset) {
// 				assert_short_circuit(
// 					t,
// 					got,
// 					false,
// 					evtparse.ShortCircuitNotApplicable,
// 				)
// 				if !got.Builder.PreferFrontendRevalidate {
// 					t.Fatal("Builder.PreferFrontendRevalidate should be true")
// 				}
// 				if !got.Builder.RunImplicitBuild {
// 					t.Fatal("Builder.RunImplicitBuild should be true")
// 				}
// 			},
// 		},
// 		{
// 			name: "chmod-only-non-empty-file-event-is-ignored",
// 			prepare_fs: func(t *testing.T) {
// 				write_file(
// 					t,
// 					filepath.Join(root_abs, "content", "non-empty.txt"),
// 					[]byte("abc"),
// 				)
// 			},
// 			raw_batch: func() []fsnotify.Event {
// 				return []fsnotify.Event{
// 					raw_evt(
// 						path(root_rel, "content", "non-empty.txt"),
// 						fsnotify.Chmod,
// 					),
// 				}
// 			},
// 			assert: func(t *testing.T, got evtparse.Workset) {
// 				assert_short_circuit(
// 					t,
// 					got,
// 					true,
// 					evtparse.ShortCircuitReasonNoOp,
// 				)
// 			},
// 		},
// 	}

// 	for _, tc := range test_cases {
// 		t.Run(tc.name, func(t *testing.T) {
// 			cfg := base_parsed_cfg(root_rel)
// 			if tc.mutate_cfg != nil {
// 				tc.mutate_cfg(cfg)
// 			}
// 			if tc.prepare_fs != nil {
// 				tc.prepare_fs(t)
// 			}
// 			got := evtparse.RawBatchToWorkset(
// 				tc.raw_batch(),
// 				cfg,
// 				tc.caller_ctx,
// 			)
// 			tc.assert(t, got)
// 		})
// 	}
// }

// func base_parsed_cfg(root strict.CWDRelPath) *config.Parsed {
// 	return &config.Parsed{
// 		ConfigPath: path(root, "wave.json"),
// 		Core: config.ParsedCore{
// 			MainAppEntry:        path(root, "main.go"),
// 			StaticPrivateDir:    path(root, "private"),
// 			StaticPublicDir:     path(root, "public"),
// 			CriticalCSSEntry:    path(root, "critical.css"),
// 			NonCriticalCSSEntry: path(root, "normal.css"),
// 		},
// 		Watch: config.ParsedWatch{},
// 	}
// }

// func assert_short_circuit(
// 	t *testing.T,
// 	got evtparse.Workset,
// 	want_should_short_circuit bool,
// 	want_reason evtparse.ShortCircuitReason,
// ) {
// 	t.Helper()
// 	if got.ShouldShortCircuit != want_should_short_circuit {
// 		t.Fatalf(
// 			"ShouldShortCircuit = %v, want %v",
// 			got.ShouldShortCircuit,
// 			want_should_short_circuit,
// 		)
// 	}
// 	if got.ShortCircuitReason != want_reason {
// 		t.Fatalf(
// 			"ShortCircuitReason = %q, want %q",
// 			got.ShortCircuitReason,
// 			want_reason,
// 		)
// 	}
// }

// func raw_evt(path strict.CWDRelPath, op fsnotify.Op) fsnotify.Event {
// 	return fsnotify.Event{
// 		Name: string(path),
// 		Op:   op,
// 	}
// }

// func path(
// 	root strict.CWDRelPath,
// 	path_parts ...string,
// ) strict.CWDRelPath {
// 	all := make([]string, 0, len(path_parts)+1)
// 	all = append(all, string(root))
// 	all = append(all, path_parts...)
// 	return strict.CWDRelPath(filepath.Clean(filepath.Join(all...)))
// }

// func glob(
// 	root strict.CWDRelPath,
// 	path_parts ...string,
// ) strict.CWDRelPath {
// 	return path(root, path_parts...)
// }

// func rel_from_cwd(t *testing.T, abs_path string) strict.CWDRelPath {
// 	t.Helper()
// 	cwd, err := os.Getwd()
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	rel, err := filepath.Rel(cwd, abs_path)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	return strict.CWDRelPath(filepath.Clean(rel))
// }

// func mk_dir(t *testing.T, path string) {
// 	t.Helper()
// 	if err := os.MkdirAll(path, 0o755); err != nil {
// 		t.Fatal(err)
// 	}
// }

// func touch_file(t *testing.T, path string) {
// 	t.Helper()
// 	write_file(t, path, nil)
// }

// func write_file(t *testing.T, path string, contents []byte) {
// 	t.Helper()
// 	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
// 		t.Fatal(err)
// 	}
// 	if err := os.WriteFile(path, contents, 0o644); err != nil {
// 		t.Fatal(err)
// 	}
// }

// func TestRawBatchToWorkset_short_circuit_none_when_not_short_circuiting(
// 	t *testing.T,
// ) {
// 	root_abs := t.TempDir()
// 	root_rel := rel_from_cwd(t, root_abs)

// 	mk_dir(t, filepath.Join(root_abs, "content", "new-dir"))
// 	cfg := base_parsed_cfg(root_rel)

// 	got := evtparse.RawBatchToWorkset(
// 		[]fsnotify.Event{
// 			raw_evt(path(root_rel, "content", "new-dir"), fsnotify.Create),
// 		},
// 		cfg,
// 		evtparse.CallerCtx{},
// 	)

// 	if got.ShouldShortCircuit {
// 		t.Fatal("ShouldShortCircuit should be false")
// 	}
// 	if got.ShortCircuitReason != evtparse.ShortCircuitNotApplicable {
// 		t.Fatalf(
// 			"ShortCircuitReason = %q, want %q",
// 			got.ShortCircuitReason,
// 			evtparse.ShortCircuitNotApplicable,
// 		)
// 	}
// }

// func TestRawBatchToWorkset_watch_new_dirs_sorted_lexicographically(
// 	t *testing.T,
// ) {
// 	root_abs := t.TempDir()
// 	root_rel := rel_from_cwd(t, root_abs)
// 	cfg := base_parsed_cfg(root_rel)

// 	mk_dir(t, filepath.Join(root_abs, "dirs", "zeta"))
// 	mk_dir(t, filepath.Join(root_abs, "dirs", "alpha"))
// 	mk_dir(t, filepath.Join(root_abs, "dirs", "beta"))

// 	got := evtparse.RawBatchToWorkset(
// 		[]fsnotify.Event{
// 			raw_evt(path(root_rel, "dirs", "zeta"), fsnotify.Create),
// 			raw_evt(path(root_rel, "dirs", "alpha"), fsnotify.Create),
// 			raw_evt(path(root_rel, "dirs", "beta"), fsnotify.Create),
// 		},
// 		cfg,
// 		evtparse.CallerCtx{},
// 	)

// 	got_string_paths := make([]string, 0, len(got.Watcher.WatchNewDirs))
// 	for _, p := range got.Watcher.WatchNewDirs {
// 		got_string_paths = append(got_string_paths, string(p))
// 	}
// 	if !slices.IsSorted(got_string_paths) {
// 		t.Fatalf("WatchNewDirs should be sorted: %#v", got.Watcher.WatchNewDirs)
// 	}
// }

// func TestRawBatchToWorkset_short_circuit_reason_stays_empty_when_false(
// 	t *testing.T,
// ) {
// 	root_abs := t.TempDir()
// 	root_rel := rel_from_cwd(t, root_abs)
// 	cfg := base_parsed_cfg(root_rel)

// 	got := evtparse.RawBatchToWorkset(
// 		[]fsnotify.Event{
// 			raw_evt(path(root_rel, "public", "asset.txt"), fsnotify.Write),
// 		},
// 		cfg,
// 		evtparse.CallerCtx{},
// 	)

// 	if got.ShouldShortCircuit {
// 		t.Fatal("ShouldShortCircuit should be false")
// 	}
// 	if strings.TrimSpace(string(got.ShortCircuitReason)) != "" {
// 		t.Fatalf(
// 			"ShortCircuitReason should be empty when not short-circuiting: %q",
// 			got.ShortCircuitReason,
// 		)
// 	}
// }
