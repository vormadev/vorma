package tooling

import (
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave"
)

func TestBuildWaveExplainReport(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.DevBuildHook = "go generate ./..."
	cfg.FrameworkDevBuildHook = "go run ./backend/cmd/build --dev --hook"
	cfg.ResolvedConfigSource = wave.NewStaticConfigSource([]byte(`{"Core":{"DistDir":"dist"}}`))
	cfg.ResolvedConfigFingerprint = "fp-test"

	report := BuildWaveExplainReport(cfg)

	expectedFragments := []string{
		"Wave Explain",
		"using_browser: true",
		"watch_root: " + root,
		"core_dev_build_hook: go generate ./...",
		"framework_dev_build_hook: go run ./backend/cmd/build --dev --hook",
		"config_source: *wave.StaticConfigSource",
		"config_fingerprint: fp-test",
	}
	for _, expectedFragment := range expectedFragments {
		if !strings.Contains(report, expectedFragment) {
			t.Fatalf("expected explain report to contain %q, report=%q", expectedFragment, report)
		}
	}
}

func TestBuildWaveDoctorReport_FindsHookMisconfiguration(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Watch.Include = []wave.WatchedFile{
		{
			Pattern: "*.go",
			OnChangeHooks: []wave.OnChangeHook{
				{
					Cmd:                             "echo hello",
					RunCombinedDevBuildHookCommands: true,
				},
			},
		},
	}

	report, hasIssues := BuildWaveDoctorReport(cfg)
	if !hasIssues {
		t.Fatal("expected doctor report to include issues")
	}
	if !strings.Contains(report, "sets both Cmd and RunCombinedDevBuildHookCommands") {
		t.Fatalf("expected hook misconfiguration issue, report=%q", report)
	}
}

func TestBuildWaveDoctorReport_FindsRunOnChangeOnlyWithoutHooks(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Watch.Include = []wave.WatchedFile{
		{
			Pattern:         "*.go",
			RunOnChangeOnly: true,
		},
	}

	report, hasIssues := BuildWaveDoctorReport(cfg)
	if !hasIssues {
		t.Fatal("expected doctor report to include issues")
	}
	if !strings.Contains(report, "RunOnChangeOnly but defines no OnChangeHooks") {
		t.Fatalf("expected run-on-change-only issue, report=%q", report)
	}
}

func TestBuildWaveDoctorReport_FindsInvalidConfigDependencyEnvName(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.ResolvedConfigSource = wave.NewStaticConfigSource(
		[]byte(`{"Core":{"MainAppEntry":"cmd/app","DistDir":"dist","ServerOnlyMode":true}}`),
	)
	cfg.ResolvedConfigDependencies = wave.ConfigProviderDependencies{
		Env: []string{"INVALID-NAME"},
	}

	report, hasIssues := BuildWaveDoctorReport(cfg)
	if !hasIssues {
		t.Fatal("expected doctor report to include issues")
	}
	if !strings.Contains(report, "is not a valid env var name") {
		t.Fatalf("expected env name issue, report=%q", report)
	}
}

func TestBuildWaveDoctorReport_NotesMissingConfigDependencyCoverage(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.ResolvedConfigSource = wave.NewStaticConfigSource(
		[]byte(`{"Core":{"MainAppEntry":"cmd/app","DistDir":"dist","ServerOnlyMode":true}}`),
	)

	report, hasIssues := BuildWaveDoctorReport(cfg)
	if hasIssues {
		t.Fatalf("expected no doctor issues, report=%q", report)
	}
	if !strings.Contains(report, "resolved config source declares no dependencies") {
		t.Fatalf("expected dependency coverage note, report=%q", report)
	}
}

func TestBuildWaveDoctorReport_ReportsNoIssuesForValidConfig(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true

	report, hasIssues := BuildWaveDoctorReport(cfg)
	if hasIssues {
		t.Fatalf("expected no doctor issues, report=%q", report)
	}
	if !strings.Contains(report, "issues: none") {
		t.Fatalf("expected no-issues marker, report=%q", report)
	}
}
