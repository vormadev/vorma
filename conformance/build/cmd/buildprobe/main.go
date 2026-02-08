package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/vormabuild"
	"github.com/vormadev/vorma/wave"
)

func main() {
	workdir := os.Getenv("BUILDPROBE_WORKDIR")
	if workdir != "" {
		if err := os.Chdir(workdir); err != nil {
			fail("chdir BUILDPROBE_WORKDIR: %v", err)
		}
	}

	configPath := os.Getenv("BUILDPROBE_CONFIG")
	if configPath == "" {
		fail("BUILDPROBE_CONFIG is required")
	}

	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		fail("read BUILDPROBE_CONFIG: %v", err)
	}

	staticRoot := os.Getenv("BUILDPROBE_STATIC_ROOT")
	if staticRoot == "" {
		staticRoot = "."
	}

	app := vorma.NewVormaApp(vorma.VormaAppConfig{
		Wave: wave.New(wave.Config{
			WaveConfigJSON: configBytes,
			DistStaticFS:   os.DirFS(staticRoot),
		}),
	})

	for _, pattern := range splitNonEmpty(os.Getenv("BUILDPROBE_SERVER_LOADERS")) {
		vorma.NewLoader(
			app,
			pattern,
			func(*vorma.LoaderReqData) (string, error) { return "ok", nil },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
	}

	markerPath := os.Getenv("BUILDPROBE_HOOK_MARKER_FILE")
	if markerPath != "" && hasArg("--hook") {
		if err := appendLine(markerPath, "framework"); err != nil {
			fail("write hook marker: %v", err)
		}
	}

	vormabuild.Build(app)

	callbackDumpPath := os.Getenv("BUILDPROBE_CALLBACK_DUMP_JSON")
	callbackPattern := os.Getenv("BUILDPROBE_RUN_CALLBACK_PATTERN")
	if callbackDumpPath != "" || callbackPattern != "" {
		payload := runWatchCallback(app.Wave.GetParsedConfig(), callbackPattern)
		if callbackDumpPath != "" {
			b, err := json.MarshalIndent(payload, "", "  ")
			if err != nil {
				fail("marshal callback payload: %v", err)
			}
			if err := os.WriteFile(callbackDumpPath, b, 0o644); err != nil {
				fail("write callback payload: %v", err)
			}
		}
	}

	dumpPath := os.Getenv("BUILDPROBE_DUMP_CONFIG_JSON")
	if dumpPath != "" {
		cfg := app.Wave.GetParsedConfig()
		watchPatterns := make([]map[string]any, 0, len(cfg.FrameworkWatchPatterns))
		for _, p := range cfg.FrameworkWatchPatterns {
			hooks := make([]map[string]any, 0, len(p.OnChangeHooks))
			for _, h := range p.OnChangeHooks {
				hooks = append(hooks, map[string]any{
					"cmd":         h.Cmd,
					"timing":      h.Timing,
					"hasCallback": h.Callback != nil,
				})
			}
			watchPatterns = append(watchPatterns, map[string]any{
				"pattern":                    p.Pattern,
				"runOnChangeOnly":            p.RunOnChangeOnly,
				"skipRebuildingNotification": p.SkipRebuildingNotification,
				"hooks":                      hooks,
			})
		}

		payload := map[string]any{
			"frameworkDevBuildHook":        cfg.FrameworkDevBuildHook,
			"frameworkProdBuildHook":       cfg.FrameworkProdBuildHook,
			"frameworkWatchPatterns":       watchPatterns,
			"frameworkIgnoredPatterns":     cfg.FrameworkIgnoredPatterns,
			"frameworkPublicFileMapOutDir": cfg.FrameworkPublicFileMapOutDir,
		}
		b, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			fail("marshal dump payload: %v", err)
		}
		if err := os.WriteFile(dumpPath, b, 0o644); err != nil {
			fail("write dump payload: %v", err)
		}
	}
}

func runWatchCallback(cfg *wave.ParsedConfig, pattern string) map[string]any {
	out := map[string]any{
		"pattern": pattern,
	}
	if pattern == "" {
		out["error"] = "BUILDPROBE_RUN_CALLBACK_PATTERN is required for callback execution"
		return out
	}

	var matched *wave.WatchedFile
	for i := range cfg.FrameworkWatchPatterns {
		if cfg.FrameworkWatchPatterns[i].Pattern == pattern {
			matched = &cfg.FrameworkWatchPatterns[i]
			break
		}
	}
	if matched == nil {
		out["error"] = "callback pattern not found"
		return out
	}

	var callback *wave.OnChangeHook
	for i := range matched.OnChangeHooks {
		if matched.OnChangeHooks[i].Callback != nil {
			callback = &matched.OnChangeHooks[i]
			break
		}
	}
	if callback == nil {
		out["error"] = "no callback hook found for pattern"
		return out
	}

	appStopped, _ := strconv.ParseBool(os.Getenv("BUILDPROBE_CALLBACK_APP_STOPPED"))
	action, err := callback.Callback(&wave.HookContext{
		FilePath:           pattern,
		AppStoppedForBatch: appStopped,
	})
	if err != nil {
		out["error"] = err.Error()
	}
	if action != nil {
		out["action"] = map[string]any{
			"reloadBrowser":  action.ReloadBrowser,
			"waitForApp":     action.WaitForApp,
			"waitForVite":    action.WaitForVite,
			"triggerRestart": action.TriggerRestart,
			"recompileGo":    action.RecompileGo,
		}
	} else {
		out["action"] = nil
	}
	return out
}

func splitNonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func hasArg(flag string) bool {
	for _, arg := range os.Args[1:] {
		if arg == flag {
			return true
		}
	}
	return false
}

func appendLine(path string, line string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString(line + "\n"); err != nil {
		return err
	}
	return nil
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(2)
}
