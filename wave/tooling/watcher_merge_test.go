package tooling

import (
	"testing"

	"github.com/vormadev/vorma/wave"
)

func TestMergeWatchedFiles_UnionSemantics(t *testing.T) {
	wfFramework := &wave.WatchedFile{
		Pattern:                            "**/*.go",
		RecompileGoBinary:                  true,
		RestartApp:                         false,
		TreatAsNonGo:                       true,
		RunOnChangeOnly:                    true,
		SkipRebuildingNotification:         true,
		OnlyRunClientDefinedRevalidateFunc: false,
		OnChangeHooks: []wave.OnChangeHook{
			{Cmd: "framework-pre", Timing: wave.OnChangeStrategyPre},
		},
	}

	wfUser := &wave.WatchedFile{
		Pattern:                            "frontend/**/*.go",
		RecompileGoBinary:                  false,
		RestartApp:                         true,
		TreatAsNonGo:                       false,
		RunOnChangeOnly:                    false,
		SkipRebuildingNotification:         false,
		OnlyRunClientDefinedRevalidateFunc: true,
		OnChangeHooks: []wave.OnChangeHook{
			{Cmd: "user-post", Timing: wave.OnChangeStrategyPost},
		},
	}

	merged := mergeWatchedFiles([]*wave.WatchedFile{wfFramework, wfUser})
	if merged == nil {
		t.Fatal("mergeWatchedFiles returned nil")
	}

	if merged.Pattern != wfFramework.Pattern {
		t.Fatalf("merged pattern = %q, want %q", merged.Pattern, wfFramework.Pattern)
	}

	if !merged.RecompileGoBinary {
		t.Fatal("expected RecompileGoBinary=true")
	}
	if !merged.RestartApp {
		t.Fatal("expected RestartApp=true")
	}
	if merged.TreatAsNonGo {
		t.Fatal("expected TreatAsNonGo=false")
	}
	if merged.RunOnChangeOnly {
		t.Fatal("expected RunOnChangeOnly=false")
	}
	if merged.SkipRebuildingNotification {
		t.Fatal("expected SkipRebuildingNotification=false")
	}
	if !merged.OnlyRunClientDefinedRevalidateFunc {
		t.Fatal("expected OnlyRunClientDefinedRevalidateFunc=true")
	}

	if len(merged.OnChangeHooks) != 2 {
		t.Fatalf("merged hooks len = %d, want 2", len(merged.OnChangeHooks))
	}
	if merged.OnChangeHooks[0].Cmd != "framework-pre" {
		t.Fatalf("hook[0] = %q, want %q", merged.OnChangeHooks[0].Cmd, "framework-pre")
	}
	if merged.OnChangeHooks[1].Cmd != "user-post" {
		t.Fatalf("hook[1] = %q, want %q", merged.OnChangeHooks[1].Cmd, "user-post")
	}

	if merged.SortedHooks == nil {
		t.Fatal("expected sorted hooks to be populated")
	}
	if len(merged.SortedHooks.Pre) != 1 || merged.SortedHooks.Pre[0].Cmd != "framework-pre" {
		t.Fatalf("unexpected sorted pre hooks: %#v", merged.SortedHooks.Pre)
	}
	if len(merged.SortedHooks.Post) != 1 || merged.SortedHooks.Post[0].Cmd != "user-post" {
		t.Fatalf("unexpected sorted post hooks: %#v", merged.SortedHooks.Post)
	}
}

func TestMergeWatchedFiles_EmptyInput(t *testing.T) {
	if got := mergeWatchedFiles(nil); got != nil {
		t.Fatalf("mergeWatchedFiles(nil) = %#v, want nil", got)
	}
}
