package build_test

import (
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildWatchReloadConformance(t *testing.T) {
	t.Run("BDC-WATCH-001_BUILD-WATCH-001_include_defaults_false_disables_default_watch_injection", func(t *testing.T) {
		includeDefaults := false
		fixture := newBuildFixture(t, &buildFixtureOptions{includeDefaults: &includeDefaults})
		dumpPath := filepath.Join(fixture.root, "probe_dump.json")

		out, err := runBuildProbeWithEnv(t, fixture, map[string]string{
			"BUILDPROBE_DUMP_CONFIG_JSON": dumpPath,
		}, "--hook", "--dev")
		if err != nil {
			t.Fatalf("build hook failed: err=%v output=%s", err, out)
		}

		dump := mustReadJSONFileMap(t, dumpPath)
		if patterns, ok := dump["frameworkWatchPatterns"].([]any); !ok || len(patterns) != 0 {
			t.Fatalf("expected no framework watch patterns when IncludeDefaults=false, got %#v", dump["frameworkWatchPatterns"])
		}
		if dump["frameworkIgnoredPatterns"] == nil {
			return
		}
		if ignored, ok := dump["frameworkIgnoredPatterns"].([]any); !ok || len(ignored) != 0 {
			t.Fatalf("expected no framework ignored patterns when IncludeDefaults=false, got %#v", dump["frameworkIgnoredPatterns"])
		}
	})

	t.Run("BDC-WATCH-002_BUILD-WATCH-002_route_defs_watch_is_run_on_change_only_with_callback", func(t *testing.T) {
		fixture := newBuildFixture(t, nil)
		dumpPath := filepath.Join(fixture.root, "probe_dump.json")

		out, err := runBuildProbeWithEnv(t, fixture, map[string]string{
			"BUILDPROBE_DUMP_CONFIG_JSON": dumpPath,
		}, "--hook", "--dev")
		if err != nil {
			t.Fatalf("build hook failed: err=%v output=%s", err, out)
		}

		dump := mustReadJSONFileMap(t, dumpPath)
		pattern := mustFindWatchPattern(t, dump["frameworkWatchPatterns"], "frontend/src/vorma.routes.ts")
		if got, _ := pattern["runOnChangeOnly"].(bool); !got {
			t.Fatalf("expected route-defs watch pattern to be RunOnChangeOnly=true, got %#v", pattern["runOnChangeOnly"])
		}
		if got, _ := pattern["skipRebuildingNotification"].(bool); !got {
			t.Fatalf("expected route-defs watch pattern to set skipRebuildingNotification=true, got %#v", pattern["skipRebuildingNotification"])
		}
		hooks := watchAnySlice(t, pattern["hooks"], "hooks")
		if len(hooks) == 0 {
			t.Fatalf("expected route-defs watch pattern to include callback hook")
		}
		firstHook := watchAnyMap(t, hooks[0], "hooks[0]")
		if got, _ := firstHook["hasCallback"].(bool); !got {
			t.Fatalf("expected route-defs watch hook to include callback")
		}
	})

	t.Run("BDC-WATCH-003_BUILD-WATCH-003_route_reload_callback_success_path_rebuilds_routes_calls_endpoint_and_requests_reload", func(t *testing.T) {
		fixture := newBuildFixture(t, nil)
		callbackDump := filepath.Join(fixture.root, "route_callback_dump.json")

		var gotPath, gotMethod string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			gotMethod = r.Method
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}))
		defer server.Close()

		port := mustPortFromURL(t, server.URL)
		out, err := runBuildProbeWithEnv(t, fixture, map[string]string{
			"BUILDPROBE_RUN_CALLBACK_PATTERN": "frontend/src/vorma.routes.ts",
			"BUILDPROBE_CALLBACK_DUMP_JSON":   callbackDump,
			"PORT":                            port,
			"WAVE_PORT_HAS_BEEN_SET":          "true",
		}, "--hook", "--dev")
		if err != nil {
			t.Fatalf("dev hook build failed: err=%v output=%s", err, out)
		}

		payload := mustReadJSONFileMap(t, callbackDump)
		if payload["error"] != nil {
			t.Fatalf("expected route callback success path without error, got %#v", payload["error"])
		}
		action := mustAnyMap(t, payload["action"], "action")
		if got, _ := action["reloadBrowser"].(bool); !got {
			t.Fatalf("expected route success action reloadBrowser=true, got %#v", action)
		}
		if got, _ := action["waitForApp"].(bool); !got {
			t.Fatalf("expected route success action waitForApp=true, got %#v", action)
		}
		if got, _ := action["waitForVite"].(bool); !got {
			t.Fatalf("expected route success action waitForVite=true, got %#v", action)
		}
		if got, _ := action["triggerRestart"].(bool); got {
			t.Fatalf("expected route success action triggerRestart=false, got %#v", action)
		}
		if got, _ := action["recompileGo"].(bool); got {
			t.Fatalf("expected route success action recompileGo=false, got %#v", action)
		}

		if gotPath != "/__vorma/reload-routes" || gotMethod != http.MethodGet {
			t.Fatalf("expected route callback to call GET /__vorma/reload-routes, got method=%q path=%q", gotMethod, gotPath)
		}

		stageOne := mustReadStageOnePathsFile(t, fixture)
		buildID, _ := stageOne["buildID"].(string)
		if !strings.HasPrefix(buildID, "dev_fast_") {
			t.Fatalf("expected route callback success path to run fast rebuild with dev_fast_* build id, got %q", buildID)
		}
	})

	t.Run("BDC-WATCH-004_BUILD-WATCH-004_route_reload_callback_fallback_requests_restart_without_recompile", func(t *testing.T) {
		fixture := newBuildFixture(t, nil)
		callbackDump := filepath.Join(fixture.root, "route_callback_dump.json")

		out, err := runBuildProbeWithEnv(t, fixture, map[string]string{
			"BUILDPROBE_RUN_CALLBACK_PATTERN": "frontend/src/vorma.routes.ts",
			"BUILDPROBE_CALLBACK_DUMP_JSON":   callbackDump,
			"PORT":                            "18081",
			"WAVE_PORT_HAS_BEEN_SET":          "true",
		}, "--hook", "--dev")
		if err != nil {
			t.Fatalf("dev hook build failed: err=%v output=%s", err, out)
		}

		payload := mustReadJSONFileMap(t, callbackDump)
		if payload["error"] != nil {
			t.Fatalf("expected route callback fallback path to return action without hard error, got %#v", payload["error"])
		}
		action := mustAnyMap(t, payload["action"], "action")
		if got, _ := action["triggerRestart"].(bool); !got {
			t.Fatalf("expected fallback action triggerRestart=true, got %#v", action)
		}
		if got, _ := action["recompileGo"].(bool); got {
			t.Fatalf("expected fallback action recompileGo=false, got %#v", action)
		}
	})

	t.Run("BDC-WATCH-005_BUILD-WATCH-005_template_reload_callback_success_path_calls_endpoint_and_requests_reload", func(t *testing.T) {
		fixture := newBuildFixture(t, nil)
		callbackDump := filepath.Join(fixture.root, "template_callback_dump.json")

		var gotPath, gotMethod string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			gotMethod = r.Method
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}))
		defer server.Close()

		port := mustPortFromURL(t, server.URL)
		out, err := runBuildProbeWithEnv(t, fixture, map[string]string{
			"BUILDPROBE_RUN_CALLBACK_PATTERN": "assets/private/index.html",
			"BUILDPROBE_CALLBACK_DUMP_JSON":   callbackDump,
			"PORT":                            port,
			"WAVE_PORT_HAS_BEEN_SET":          "true",
		}, "--hook", "--dev")
		if err != nil {
			t.Fatalf("dev hook build failed: err=%v output=%s", err, out)
		}

		payload := mustReadJSONFileMap(t, callbackDump)
		if payload["error"] != nil {
			t.Fatalf("expected template callback success path without error, got %#v", payload["error"])
		}
		action := mustAnyMap(t, payload["action"], "action")
		if got, _ := action["reloadBrowser"].(bool); !got {
			t.Fatalf("expected template success action reloadBrowser=true, got %#v", action)
		}
		if got, _ := action["waitForApp"].(bool); !got {
			t.Fatalf("expected template success action waitForApp=true, got %#v", action)
		}
		if got, _ := action["waitForVite"].(bool); !got {
			t.Fatalf("expected template success action waitForVite=true, got %#v", action)
		}
		if got, _ := action["triggerRestart"].(bool); got {
			t.Fatalf("expected template success action triggerRestart=false, got %#v", action)
		}
		if got, _ := action["recompileGo"].(bool); got {
			t.Fatalf("expected template success action recompileGo=false, got %#v", action)
		}
		if gotPath != "/__vorma/reload-template" || gotMethod != http.MethodGet {
			t.Fatalf("expected template callback to call GET /__vorma/reload-template, got method=%q path=%q", gotMethod, gotPath)
		}
	})

	t.Run("BDC-WATCH-006_BUILD-WATCH-006_template_reload_callback_fallback_requests_restart_without_recompile", func(t *testing.T) {
		fixture := newBuildFixture(t, nil)
		callbackDump := filepath.Join(fixture.root, "template_callback_dump.json")

		out, err := runBuildProbeWithEnv(t, fixture, map[string]string{
			"BUILDPROBE_RUN_CALLBACK_PATTERN": "assets/private/index.html",
			"BUILDPROBE_CALLBACK_DUMP_JSON":   callbackDump,
			"PORT":                            "18082",
			"WAVE_PORT_HAS_BEEN_SET":          "true",
		}, "--hook", "--dev")
		if err != nil {
			t.Fatalf("dev hook build failed: err=%v output=%s", err, out)
		}

		payload := mustReadJSONFileMap(t, callbackDump)
		if payload["error"] != nil {
			t.Fatalf("expected template callback fallback path to return action without hard error, got %#v", payload["error"])
		}
		action := mustAnyMap(t, payload["action"], "action")
		if got, _ := action["triggerRestart"].(bool); !got {
			t.Fatalf("expected template fallback action triggerRestart=true, got %#v", action)
		}
		if got, _ := action["recompileGo"].(bool); got {
			t.Fatalf("expected template fallback action recompileGo=false, got %#v", action)
		}
	})

	t.Run("BDC-WATCH-007_BUILD-WATCH-007_go_watch_uses_dev_build_hook_with_concurrent_timing", func(t *testing.T) {
		fixture := newBuildFixture(t, nil)
		dumpPath := filepath.Join(fixture.root, "probe_dump.json")

		out, err := runBuildProbeWithEnv(t, fixture, map[string]string{
			"BUILDPROBE_DUMP_CONFIG_JSON": dumpPath,
		}, "--hook", "--dev")
		if err != nil {
			t.Fatalf("build hook failed: err=%v output=%s", err, out)
		}

		dump := mustReadJSONFileMap(t, dumpPath)
		pattern := mustFindWatchPattern(t, dump["frameworkWatchPatterns"], "**/*.go")
		hooks := watchAnySlice(t, pattern["hooks"], "hooks")
		if len(hooks) == 0 {
			t.Fatalf("expected go watch pattern to include hook")
		}
		hook := watchAnyMap(t, hooks[0], "hooks[0]")
		if got, _ := hook["cmd"].(string); got != "DevBuildHook" {
			t.Fatalf("expected go watch hook cmd=%q, got %#v", "DevBuildHook", hook["cmd"])
		}
		if got, _ := hook["timing"].(string); got != "concurrent" {
			t.Fatalf("expected go watch hook timing=%q, got %#v", "concurrent", hook["timing"])
		}
	})

	t.Run("BDC-WATCH-008_BUILD-WATCH-008_generated_ts_outputs_are_in_framework_ignore_patterns", func(t *testing.T) {
		fixture := newBuildFixture(t, nil)
		dumpPath := filepath.Join(fixture.root, "probe_dump.json")

		out, err := runBuildProbeWithEnv(t, fixture, map[string]string{
			"BUILDPROBE_DUMP_CONFIG_JSON": dumpPath,
		}, "--hook", "--dev")
		if err != nil {
			t.Fatalf("build hook failed: err=%v output=%s", err, out)
		}

		dump := mustReadJSONFileMap(t, dumpPath)
		if got, _ := dump["frameworkPublicFileMapOutDir"].(string); got != "frontend/src/vorma.gen" {
			t.Fatalf("expected framework public filemap out dir to equal TSGenOutDir, got %q", got)
		}
		ignored := watchStringSliceAny(t, dump["frameworkIgnoredPatterns"], "frameworkIgnoredPatterns")
		required := []string{
			"frontend/src/vorma.gen/index.ts",
			"frontend/src/vorma.gen/filemap.ts",
			"frontend/src/vorma.gen/filemap.json",
		}
		for _, needle := range required {
			if !containsString(ignored, needle) {
				t.Fatalf("expected framework ignored patterns to include %q, got %v", needle, ignored)
			}
		}
	})
}

func mustFindWatchPattern(t *testing.T, raw any, pattern string) map[string]any {
	t.Helper()
	items := watchAnySlice(t, raw, "frameworkWatchPatterns")
	for _, item := range items {
		m := watchAnyMap(t, item, "frameworkWatchPatterns[]")
		if got, _ := m["pattern"].(string); got == pattern {
			return m
		}
	}
	t.Fatalf("expected watch pattern %q, got %#v", pattern, raw)
	return nil
}

func watchAnySlice(t *testing.T, raw any, field string) []any {
	t.Helper()
	s, ok := raw.([]any)
	if !ok {
		t.Fatalf("expected %s as []any, got %#v", field, raw)
	}
	return s
}

func watchAnyMap(t *testing.T, raw any, field string) map[string]any {
	t.Helper()
	m, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("expected %s as map[string]any, got %#v", field, raw)
	}
	return m
}

func watchStringSliceAny(t *testing.T, raw any, field string) []string {
	t.Helper()
	items := watchAnySlice(t, raw, field)
	out := make([]string, 0, len(items))
	for i, item := range items {
		s, ok := item.(string)
		if !ok {
			t.Fatalf("expected %s[%d] as string, got %#v", field, i, item)
		}
		out = append(out, s)
	}
	return out
}

func containsString(values []string, needle string) bool {
	for _, v := range values {
		if strings.TrimSpace(v) == needle {
			return true
		}
	}
	return false
}

func mustPortFromURL(t *testing.T, rawURL string) string {
	t.Helper()
	hostPort := strings.TrimPrefix(rawURL, "http://")
	_, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		t.Fatalf("split host/port from %q: %v", rawURL, err)
	}
	return port
}
