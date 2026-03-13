package config

// import (
// 	"encoding/json"
// 	"fmt"
// 	"os"
// 	"path/filepath"
// 	"testing"

// 	"github.com/vormadev/vorma/kit/strict"
// )

// /////////////////////////////////////////////////////////////////////
// /////// Helpers
// /////////////////////////////////////////////////////////////////////

// func touch_file(t *testing.T, path string) {
// 	t.Helper()
// 	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
// 		t.Fatal(err)
// 	}
// 	if err := os.WriteFile(path, nil, 0o644); err != nil {
// 		t.Fatal(err)
// 	}
// }

// func mk_dir(t *testing.T, path string) {
// 	t.Helper()
// 	if err := os.MkdirAll(path, 0o755); err != nil {
// 		t.Fatal(err)
// 	}
// }

// func rel_from_cwd(t *testing.T, abs_path string) string {
// 	t.Helper()
// 	cwd, err := os.Getwd()
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	rel, err := filepath.Rel(cwd, abs_path)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	return rel
// }

// func must_panic(t *testing.T, label string, f func()) {
// 	t.Helper()
// 	defer func() {
// 		if recover() == nil {
// 			t.Fatalf("%s: expected panic, but did not panic", label)
// 		}
// 	}()
// 	f()
// }

// func must_not_panic(t *testing.T, label string, f func()) {
// 	t.Helper()
// 	defer func() {
// 		if r := recover(); r != nil {
// 			t.Fatalf("%s: unexpected panic: %v", label, r)
// 		}
// 	}()
// 	f()
// }

// func expect_path(
// 	t *testing.T,
// 	label string,
// 	got strict.CWDRelPath,
// 	rel_root string,
// 	suffix string,
// ) {
// 	t.Helper()
// 	want := strict.CWDRelPath(filepath.Clean(filepath.Join(rel_root, suffix)))
// 	if got != want {
// 		t.Fatalf("%s: got %q, want %q", label, got, want)
// 	}
// }

// // full_project_config returns a Raw whose fields reference paths that
// // full_project_fs will create under root.
// func full_project_config(t *testing.T, root string) Raw {
// 	t.Helper()
// 	cfg_file := filepath.Join(root, "wave.json")
// 	touch_file(t, cfg_file)
// 	return Raw{
// 		ConfigPath: strict.CWDRelPath(rel_from_cwd(t, cfg_file)),
// 		Core: RawCore{
// 			ResolveRoot:       ".",
// 			MainAppEntry:      "main.go",
// 			DevBuildHook:      "make dev",
// 			ProdBuildHook:     "make prod",
// 			ServerOnlyMode:    false,
// 			SequentialGoBuild: true,
// 			PublicPathPrefix:  "/static/",
// 			StaticAssetDirs: RawStaticAssetDirs{
// 				Private: "private",
// 				Public:  "public",
// 			},
// 			CSSEntryFiles: RawCSSEntryFiles{
// 				Critical:    "critical.css",
// 				NonCritical: "noncritical.css",
// 			},
// 		},
// 		Vite: RawVite{
// 			JSPackageManagerBaseCmd: "pnpm",
// 			JSPackageManagerCmdDir:  "jspkgdir",
// 			DefaultPort:             3000,
// 			ViteConfigFile:          "vite.config.ts",
// 		},
// 		Watch: RawWatch{
// 			HealthcheckEndpoint: "/healthz",
// 			Include: []RawIncludeEntry{
// 				{
// 					Pattern:           "src/**/*.go",
// 					RecompileGoBinary: true,
// 					RestartApp:        true,
// 					OnChangeHooks: []RawOnChangeHook{
// 						{
// 							Cmd:     "go generate ./...",
// 							Timing:  "pre",
// 							Exclude: []string{"src/vendor/**"},
// 						},
// 						{
// 							Cmd:    "echo concurrent",
// 							Timing: "concurrent",
// 						},
// 						{
// 							Cmd:    "echo no-wait",
// 							Timing: "concurrent-no-wait",
// 						},
// 						{
// 							Cmd:    "echo post",
// 							Timing: "post",
// 						},
// 					},
// 				},
// 				{
// 					Pattern:                            "assets/**/*.md",
// 					OnlyRunClientDefinedRevalidateFunc: true,
// 					SkipRebuildingNotification:         true,
// 					RunOnChangeOnly:                    true,
// 					TreatAsNonGo:                       true,
// 				},
// 			},
// 			Exclude: []string{"tmp/**", "dist/**"},
// 		},
// 	}
// }

// // full_project_fs creates all the dirs and files that full_project_config
// // references under root.
// func full_project_fs(t *testing.T, root string) {
// 	t.Helper()
// 	touch_file(t, filepath.Join(root, "main.go"))
// 	mk_dir(t, filepath.Join(root, "private"))
// 	mk_dir(t, filepath.Join(root, "public"))
// 	touch_file(t, filepath.Join(root, "critical.css"))
// 	touch_file(t, filepath.Join(root, "noncritical.css"))
// 	mk_dir(t, filepath.Join(root, "jspkgdir"))
// 	touch_file(t, filepath.Join(root, "vite.config.ts"))
// }

// // minimal_server_only_raw returns a minimal server-only Raw config.
// func minimal_server_only_raw(t *testing.T, root string) Raw {
// 	t.Helper()
// 	cfg_file := filepath.Join(root, "wave.json")
// 	touch_file(t, cfg_file)
// 	touch_file(t, filepath.Join(root, "main.go"))
// 	return Raw{
// 		ConfigPath: strict.CWDRelPath(rel_from_cwd(t, cfg_file)),
// 		Core: RawCore{
// 			ResolveRoot:    ".",
// 			MainAppEntry:   "main.go",
// 			ServerOnlyMode: true,
// 		},
// 	}
// }

// /////////////////////////////////////////////////////////////////////
// /////// ConfigPathToRaw
// /////////////////////////////////////////////////////////////////////

// func TestConfigPathToRaw_valid(t *testing.T) {
// 	root := t.TempDir()
// 	cfg_file := filepath.Join(root, "wave.json")

// 	raw_input := Raw{
// 		Core: RawCore{
// 			ResolveRoot:  ".",
// 			MainAppEntry: "main.go",
// 		},
// 		Watch: RawWatch{
// 			HealthcheckEndpoint: "/healthz",
// 		},
// 	}
// 	b, err := json.Marshal(raw_input)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if err := os.WriteFile(cfg_file, b, 0o644); err != nil {
// 		t.Fatal(err)
// 	}

// 	rel_cfg := rel_from_cwd(t, cfg_file)
// 	got := ConfigPathToRaw(rel_cfg)

// 	if got.ConfigPath != strict.CWDRelPath(filepath.Clean(rel_cfg)) {
// 		t.Fatalf(
// 			"ConfigPath: got %q, want %q",
// 			got.ConfigPath,
// 			filepath.Clean(rel_cfg),
// 		)
// 	}
// 	if got.Core.MainAppEntry != "main.go" {
// 		t.Fatalf(
// 			"Core.MainAppEntry: got %q, want %q",
// 			got.Core.MainAppEntry,
// 			"main.go",
// 		)
// 	}
// 	if got.Watch.HealthcheckEndpoint != "/healthz" {
// 		t.Fatalf(
// 			"Watch.HealthcheckEndpoint: got %q, want %q",
// 			got.Watch.HealthcheckEndpoint,
// 			"/healthz",
// 		)
// 	}
// }

// func TestConfigPathToRaw_absolute_path_panics(t *testing.T) {
// 	must_panic(t, "absolute path", func() {
// 		ConfigPathToRaw("/some/absolute/path.json")
// 	})
// }

// func TestConfigPathToRaw_nonexistent_file_panics(t *testing.T) {
// 	must_panic(t, "nonexistent file", func() {
// 		ConfigPathToRaw("nonexistent_dir_abc123/wave.json")
// 	})
// }

// func TestConfigPathToRaw_directory_panics(t *testing.T) {
// 	root := t.TempDir()
// 	mk_dir(t, filepath.Join(root, "notafile"))
// 	rel := rel_from_cwd(t, filepath.Join(root, "notafile"))
// 	must_panic(t, "directory instead of file", func() {
// 		ConfigPathToRaw(rel)
// 	})
// }

// func TestConfigPathToRaw_invalid_json_panics(t *testing.T) {
// 	root := t.TempDir()
// 	cfg_file := filepath.Join(root, "wave.json")
// 	if err := os.WriteFile(cfg_file, []byte("{not json!!!"), 0o644); err != nil {
// 		t.Fatal(err)
// 	}
// 	rel := rel_from_cwd(t, cfg_file)
// 	must_panic(t, "invalid json", func() {
// 		ConfigPathToRaw(rel)
// 	})
// }

// func TestConfigPathToRaw_wrong_types_panics(t *testing.T) {
// 	root := t.TempDir()
// 	cfg_file := filepath.Join(root, "wave.json")
// 	if err := os.WriteFile(cfg_file, []byte(`{"Core": "wrong"}`), 0o644); err != nil {
// 		t.Fatal(err)
// 	}
// 	rel := rel_from_cwd(t, cfg_file)
// 	must_panic(t, "wrong type for Core", func() {
// 		ConfigPathToRaw(rel)
// 	})
// }

// /////////////////////////////////////////////////////////////////////
// /////// RawToParsed — Happy Paths
// /////////////////////////////////////////////////////////////////////

// func TestRawToParsed_full_config(t *testing.T) {
// 	root := t.TempDir()
// 	full_project_fs(t, root)
// 	raw := full_project_config(t, root)
// 	parsed := RawToParsed(&raw)

// 	rel_root := rel_from_cwd(t, root)

// 	// Core paths
// 	expect_path(
// 		t,
// 		"MainAppEntry",
// 		parsed.Core.MainAppEntry,
// 		rel_root,
// 		"main.go",
// 	)
// 	expect_path(
// 		t,
// 		"StaticPrivateDir",
// 		parsed.Core.StaticPrivateDir,
// 		rel_root,
// 		"private",
// 	)
// 	expect_path(
// 		t,
// 		"StaticPublicDir",
// 		parsed.Core.StaticPublicDir,
// 		rel_root,
// 		"public",
// 	)
// 	expect_path(
// 		t,
// 		"CriticalCSSEntry",
// 		parsed.Core.CriticalCSSEntry,
// 		rel_root,
// 		"critical.css",
// 	)
// 	expect_path(
// 		t,
// 		"NonCriticalCSSEntry",
// 		parsed.Core.NonCriticalCSSEntry,
// 		rel_root,
// 		"noncritical.css",
// 	)

// 	// Core scalars
// 	if parsed.Core.PublicPathPrefix != "/static/" {
// 		t.Fatalf("PublicPathPrefix: got %q", parsed.Core.PublicPathPrefix)
// 	}
// 	if parsed.Core.ServerOnlyMode != false {
// 		t.Fatal("ServerOnlyMode should be false")
// 	}
// 	if parsed.Core.SequentialGoBuild != true {
// 		t.Fatal("SequentialGoBuild should be true")
// 	}
// 	if parsed.Core.DevBuildHook != "make dev" {
// 		t.Fatalf("DevBuildHook: got %q", parsed.Core.DevBuildHook)
// 	}
// 	if parsed.Core.ProdBuildHook != "make prod" {
// 		t.Fatalf("ProdBuildHook: got %q", parsed.Core.ProdBuildHook)
// 	}

// 	// Vite
// 	if parsed.Vite.JSPackageManagerBaseCmd != "pnpm" {
// 		t.Fatalf(
// 			"JSPackageManagerBaseCmd: got %q",
// 			parsed.Vite.JSPackageManagerBaseCmd,
// 		)
// 	}
// 	expect_path(
// 		t,
// 		"JSPackageManagerCmdDir",
// 		parsed.Vite.JSPackageManagerCmdDir,
// 		rel_root,
// 		"jspkgdir",
// 	)
// 	if parsed.Vite.DefaultPort != 3000 {
// 		t.Fatalf("DefaultPort: got %d", parsed.Vite.DefaultPort)
// 	}
// 	expect_path(
// 		t,
// 		"ViteConfigFile",
// 		parsed.Vite.ViteConfigFile,
// 		rel_root,
// 		"vite.config.ts",
// 	)

// 	// Watch
// 	if parsed.Watch.HealthcheckEndpoint != "/healthz" {
// 		t.Fatalf(
// 			"HealthcheckEndpoint: got %q",
// 			parsed.Watch.HealthcheckEndpoint,
// 		)
// 	}

// 	// Watch.Include
// 	if len(parsed.Watch.Include) != 2 {
// 		t.Fatalf(
// 			"Watch.Include length: got %d, want 2",
// 			len(parsed.Watch.Include),
// 		)
// 	}

// 	inc0 := parsed.Watch.Include[0]
// 	expect_path(t, "Include[0].Pattern", inc0.Pattern, rel_root, "src/**/*.go")
// 	if !inc0.RecompileGoBinary {
// 		t.Fatal("Include[0].RecompileGoBinary should be true")
// 	}
// 	if !inc0.RestartApp {
// 		t.Fatal("Include[0].RestartApp should be true")
// 	}
// 	if len(inc0.OnChangeHooks) != 4 {
// 		t.Fatalf(
// 			"Include[0].OnChangeHooks length: got %d, want 4",
// 			len(inc0.OnChangeHooks),
// 		)
// 	}
// 	if inc0.OnChangeHooks[0].Cmd != "go generate ./..." {
// 		t.Fatalf(
// 			"Include[0].OnChangeHooks[0].Cmd: got %q",
// 			inc0.OnChangeHooks[0].Cmd,
// 		)
// 	}
// 	if inc0.OnChangeHooks[0].Timing != OnChangeHookTimingPre {
// 		t.Fatalf(
// 			"Include[0].OnChangeHooks[0].Timing: got %q",
// 			inc0.OnChangeHooks[0].Timing,
// 		)
// 	}
// 	if len(inc0.OnChangeHooks[0].Exclude) != 1 {
// 		t.Fatalf(
// 			"Include[0].OnChangeHooks[0].Exclude length: got %d",
// 			len(inc0.OnChangeHooks[0].Exclude),
// 		)
// 	}
// 	expect_path(t, "Include[0].OnChangeHooks[0].Exclude[0]",
// 		inc0.OnChangeHooks[0].Exclude[0], rel_root, "src/vendor/**")
// 	if inc0.OnChangeHooks[1].Timing != OnChangeHookTimingConcurrent {
// 		t.Fatalf(
// 			"Include[0].OnChangeHooks[1].Timing: got %q",
// 			inc0.OnChangeHooks[1].Timing,
// 		)
// 	}
// 	if inc0.OnChangeHooks[2].Timing != OnChangeHookTimingConcurrentNoWait {
// 		t.Fatalf(
// 			"Include[0].OnChangeHooks[2].Timing: got %q",
// 			inc0.OnChangeHooks[2].Timing,
// 		)
// 	}
// 	if inc0.OnChangeHooks[3].Timing != OnChangeHookTimingPost {
// 		t.Fatalf(
// 			"Include[0].OnChangeHooks[3].Timing: got %q",
// 			inc0.OnChangeHooks[3].Timing,
// 		)
// 	}

// 	inc1 := parsed.Watch.Include[1]
// 	expect_path(
// 		t,
// 		"Include[1].Pattern",
// 		inc1.Pattern,
// 		rel_root,
// 		"assets/**/*.md",
// 	)
// 	if !inc1.OnlyRunClientDefinedRevalidateFunc {
// 		t.Fatal("Include[1].OnlyRunClientDefinedRevalidateFunc should be true")
// 	}
// 	if !inc1.SkipRebuildingNotification {
// 		t.Fatal("Include[1].SkipRebuildingNotification should be true")
// 	}
// 	if !inc1.RunOnChangeOnly {
// 		t.Fatal("Include[1].RunOnChangeOnly should be true")
// 	}
// 	if !inc1.TreatAsNonGo {
// 		t.Fatal("Include[1].TreatAsNonGo should be true")
// 	}

// 	// Watch.Exclude
// 	if len(parsed.Watch.Exclude) != 2 {
// 		t.Fatalf(
// 			"Watch.Exclude length: got %d, want 2",
// 			len(parsed.Watch.Exclude),
// 		)
// 	}
// 	expect_path(
// 		t,
// 		"Watch.Exclude[0]",
// 		parsed.Watch.Exclude[0],
// 		rel_root,
// 		"tmp/**",
// 	)
// 	expect_path(
// 		t,
// 		"Watch.Exclude[1]",
// 		parsed.Watch.Exclude[1],
// 		rel_root,
// 		"dist/**",
// 	)
// }

// func TestRawToParsed_server_only_mode(t *testing.T) {
// 	root := t.TempDir()
// 	raw := minimal_server_only_raw(t, root)
// 	parsed := RawToParsed(&raw)

// 	if !parsed.Core.ServerOnlyMode {
// 		t.Fatal("ServerOnlyMode should be true")
// 	}
// 	// full-stack-only fields should be zero-value
// 	if parsed.Core.PublicPathPrefix != "" {
// 		t.Fatalf(
// 			"PublicPathPrefix should be empty, got %q",
// 			parsed.Core.PublicPathPrefix,
// 		)
// 	}
// 	if parsed.Core.StaticPrivateDir != "" {
// 		t.Fatalf(
// 			"StaticPrivateDir should be empty, got %q",
// 			parsed.Core.StaticPrivateDir,
// 		)
// 	}
// 	if parsed.Core.StaticPublicDir != "" {
// 		t.Fatalf(
// 			"StaticPublicDir should be empty, got %q",
// 			parsed.Core.StaticPublicDir,
// 		)
// 	}
// 	if parsed.Core.CriticalCSSEntry != "" {
// 		t.Fatalf(
// 			"CriticalCSSEntry should be empty, got %q",
// 			parsed.Core.CriticalCSSEntry,
// 		)
// 	}
// 	if parsed.Core.NonCriticalCSSEntry != "" {
// 		t.Fatalf(
// 			"NonCriticalCSSEntry should be empty, got %q",
// 			parsed.Core.NonCriticalCSSEntry,
// 		)
// 	}
// }

// func TestRawToParsed_no_vite(t *testing.T) {
// 	root := t.TempDir()
// 	raw := minimal_server_only_raw(t, root)
// 	parsed := RawToParsed(&raw)

// 	if parsed.Vite.JSPackageManagerBaseCmd != "" {
// 		t.Fatalf(
// 			"JSPackageManagerBaseCmd should be empty, got %q",
// 			parsed.Vite.JSPackageManagerBaseCmd,
// 		)
// 	}
// 	if parsed.Vite.DefaultPort != 0 {
// 		t.Fatalf("DefaultPort should be 0, got %d", parsed.Vite.DefaultPort)
// 	}
// }

// func TestRawToParsed_vite_default_port(t *testing.T) {
// 	root := t.TempDir()
// 	raw := minimal_server_only_raw(t, root)
// 	mk_dir(t, filepath.Join(root, "jspkgdir"))
// 	raw.Vite = RawVite{
// 		JSPackageManagerBaseCmd: "pnpm",
// 		JSPackageManagerCmdDir:  "jspkgdir",
// 		DefaultPort:             0, // should default to 5173
// 	}
// 	parsed := RawToParsed(&raw)
// 	if parsed.Vite.DefaultPort != 5173 {
// 		t.Fatalf("DefaultPort: got %d, want 5173", parsed.Vite.DefaultPort)
// 	}
// }

// func TestRawToParsed_resolve_root_parent_dir(t *testing.T) {
// 	root := t.TempDir()
// 	sub_dir := filepath.Join(root, "sub")
// 	mk_dir(t, sub_dir)
// 	touch_file(t, filepath.Join(root, "main.go"))

// 	cfg_file := filepath.Join(sub_dir, "wave.json")
// 	touch_file(t, cfg_file)

// 	raw := Raw{
// 		ConfigPath: strict.CWDRelPath(rel_from_cwd(t, cfg_file)),
// 		Core: RawCore{
// 			ResolveRoot:    "../",
// 			MainAppEntry:   "main.go",
// 			ServerOnlyMode: true,
// 		},
// 	}
// 	parsed := RawToParsed(&raw)
// 	expect_path(
// 		t,
// 		"MainAppEntry",
// 		parsed.Core.MainAppEntry,
// 		rel_from_cwd(t, root),
// 		"main.go",
// 	)
// }

// func TestRawToParsed_public_path_prefix_normalization(t *testing.T) {
// 	root := t.TempDir()
// 	full_project_fs(t, root)
// 	raw := full_project_config(t, root)

// 	cases := []struct {
// 		input string
// 		want  string
// 	}{
// 		{"static", "/static/"},
// 		{"/static", "/static/"},
// 		{"static/", "/static/"},
// 		{"/static/", "/static/"},
// 		{"  /static/  ", "/static/"},
// 		{"/", "/"},
// 	}
// 	for _, tc := range cases {
// 		raw.Core.PublicPathPrefix = tc.input
// 		parsed := RawToParsed(&raw)
// 		if parsed.Core.PublicPathPrefix != tc.want {
// 			t.Fatalf(
// 				"PublicPathPrefix input %q: got %q, want %q",
// 				tc.input,
// 				parsed.Core.PublicPathPrefix,
// 				tc.want,
// 			)
// 		}
// 	}
// }

// func TestRawToParsed_healthcheck_endpoint_gets_leading_slash(t *testing.T) {
// 	root := t.TempDir()
// 	raw := minimal_server_only_raw(t, root)
// 	raw.Watch.HealthcheckEndpoint = "healthz"
// 	parsed := RawToParsed(&raw)
// 	if parsed.Watch.HealthcheckEndpoint != "/healthz" {
// 		t.Fatalf(
// 			"HealthcheckEndpoint: got %q, want %q",
// 			parsed.Watch.HealthcheckEndpoint,
// 			"/healthz",
// 		)
// 	}
// }

// func TestRawToParsed_whitespace_trimming(t *testing.T) {
// 	root := t.TempDir()
// 	raw := minimal_server_only_raw(t, root)
// 	raw.Core.DevBuildHook = "  make dev  "
// 	raw.Core.ProdBuildHook = "  make prod  "
// 	parsed := RawToParsed(&raw)
// 	if parsed.Core.DevBuildHook != "make dev" {
// 		t.Fatalf("DevBuildHook: got %q", parsed.Core.DevBuildHook)
// 	}
// 	if parsed.Core.ProdBuildHook != "make prod" {
// 		t.Fatalf("ProdBuildHook: got %q", parsed.Core.ProdBuildHook)
// 	}
// }

// func TestRawToParsed_empty_hooks_are_allowed(t *testing.T) {
// 	root := t.TempDir()
// 	raw := minimal_server_only_raw(t, root)
// 	raw.Core.DevBuildHook = ""
// 	raw.Core.ProdBuildHook = "  "
// 	must_not_panic(t, "empty hooks", func() {
// 		parsed := RawToParsed(&raw)
// 		if parsed.Core.DevBuildHook != "" {
// 			t.Fatalf("DevBuildHook: got %q", parsed.Core.DevBuildHook)
// 		}
// 		if parsed.Core.ProdBuildHook != "" {
// 			t.Fatalf("ProdBuildHook: got %q", parsed.Core.ProdBuildHook)
// 		}
// 	})
// }

// /////////////////////////////////////////////////////////////////////
// /////// RawToParsed — Panics
// /////////////////////////////////////////////////////////////////////

// func TestRawToParsed_nil_raw_panics(t *testing.T) {
// 	must_panic(t, "nil raw", func() {
// 		RawToParsed(nil)
// 	})
// }

// func TestRawToParsed_resolve_root_not_a_dir_panics(t *testing.T) {
// 	root := t.TempDir()
// 	cfg_file := filepath.Join(root, "wave.json")
// 	touch_file(t, cfg_file)
// 	touch_file(t, filepath.Join(root, "notadir"))
// 	raw := Raw{
// 		ConfigPath: strict.CWDRelPath(rel_from_cwd(t, cfg_file)),
// 		Core: RawCore{
// 			ResolveRoot:    "notadir",
// 			MainAppEntry:   "main.go",
// 			ServerOnlyMode: true,
// 		},
// 	}
// 	must_panic(t, "resolve root is a file", func() {
// 		RawToParsed(&raw)
// 	})
// }

// func TestRawToParsed_main_app_entry_not_a_file_panics(t *testing.T) {
// 	root := t.TempDir()
// 	cfg_file := filepath.Join(root, "wave.json")
// 	touch_file(t, cfg_file)
// 	mk_dir(t, filepath.Join(root, "somedir"))
// 	raw := Raw{
// 		ConfigPath: strict.CWDRelPath(rel_from_cwd(t, cfg_file)),
// 		Core: RawCore{
// 			ResolveRoot:    ".",
// 			MainAppEntry:   "somedir",
// 			ServerOnlyMode: true,
// 		},
// 	}
// 	must_panic(t, "main app entry is a dir", func() {
// 		RawToParsed(&raw)
// 	})
// }

// func TestRawToParsed_main_app_entry_does_not_exist_panics(t *testing.T) {
// 	root := t.TempDir()
// 	cfg_file := filepath.Join(root, "wave.json")
// 	touch_file(t, cfg_file)
// 	raw := Raw{
// 		ConfigPath: strict.CWDRelPath(rel_from_cwd(t, cfg_file)),
// 		Core: RawCore{
// 			ResolveRoot:    ".",
// 			MainAppEntry:   "nope.go",
// 			ServerOnlyMode: true,
// 		},
// 	}
// 	must_panic(t, "main app entry missing", func() {
// 		RawToParsed(&raw)
// 	})
// }

// func TestRawToParsed_absolute_main_app_entry_panics(t *testing.T) {
// 	root := t.TempDir()
// 	cfg_file := filepath.Join(root, "wave.json")
// 	touch_file(t, cfg_file)
// 	raw := Raw{
// 		ConfigPath: strict.CWDRelPath(rel_from_cwd(t, cfg_file)),
// 		Core: RawCore{
// 			ResolveRoot:    ".",
// 			MainAppEntry:   "/absolute/main.go",
// 			ServerOnlyMode: true,
// 		},
// 	}
// 	must_panic(t, "absolute main app entry", func() {
// 		RawToParsed(&raw)
// 	})
// }

// func TestRawToParsed_static_private_dir_is_file_panics(t *testing.T) {
// 	root := t.TempDir()
// 	full_project_fs(t, root)
// 	raw := full_project_config(t, root)
// 	// replace private dir with a file
// 	os.RemoveAll(filepath.Join(root, "private"))
// 	touch_file(t, filepath.Join(root, "private"))
// 	must_panic(t, "private dir is a file", func() {
// 		RawToParsed(&raw)
// 	})
// }

// func TestRawToParsed_static_public_dir_is_file_panics(t *testing.T) {
// 	root := t.TempDir()
// 	full_project_fs(t, root)
// 	raw := full_project_config(t, root)
// 	os.RemoveAll(filepath.Join(root, "public"))
// 	touch_file(t, filepath.Join(root, "public"))
// 	must_panic(t, "public dir is a file", func() {
// 		RawToParsed(&raw)
// 	})
// }

// func TestRawToParsed_critical_css_is_dir_panics(t *testing.T) {
// 	root := t.TempDir()
// 	full_project_fs(t, root)
// 	raw := full_project_config(t, root)
// 	os.Remove(filepath.Join(root, "critical.css"))
// 	mk_dir(t, filepath.Join(root, "critical.css"))
// 	must_panic(t, "critical css is a dir", func() {
// 		RawToParsed(&raw)
// 	})
// }

// func TestRawToParsed_non_critical_css_is_dir_panics(t *testing.T) {
// 	root := t.TempDir()
// 	full_project_fs(t, root)
// 	raw := full_project_config(t, root)
// 	os.Remove(filepath.Join(root, "noncritical.css"))
// 	mk_dir(t, filepath.Join(root, "noncritical.css"))
// 	must_panic(t, "noncritical css is a dir", func() {
// 		RawToParsed(&raw)
// 	})
// }

// func TestRawToParsed_duplicate_reserved_path_panics(t *testing.T) {
// 	root := t.TempDir()
// 	full_project_fs(t, root)
// 	raw := full_project_config(t, root)
// 	// make critical and noncritical point to same file
// 	raw.Core.CSSEntryFiles = RawCSSEntryFiles{
// 		Critical:    "critical.css",
// 		NonCritical: "critical.css",
// 	}
// 	must_panic(t, "duplicate reserved path", func() {
// 		RawToParsed(&raw)
// 	})
// }

// func TestRawToParsed_vite_empty_base_cmd_panics(t *testing.T) {
// 	root := t.TempDir()
// 	raw := minimal_server_only_raw(t, root)
// 	mk_dir(t, filepath.Join(root, "jspkgdir"))
// 	raw.Vite = RawVite{
// 		JSPackageManagerBaseCmd: "  ",
// 		JSPackageManagerCmdDir:  "jspkgdir",
// 	}
// 	must_panic(t, "empty vite base cmd", func() {
// 		RawToParsed(&raw)
// 	})
// }

// func TestRawToParsed_vite_cmd_dir_does_not_exist_panics(t *testing.T) {
// 	root := t.TempDir()
// 	raw := minimal_server_only_raw(t, root)
// 	raw.Vite = RawVite{
// 		JSPackageManagerBaseCmd: "pnpm",
// 		JSPackageManagerCmdDir:  "nope",
// 	}
// 	must_panic(t, "vite cmd dir missing", func() {
// 		RawToParsed(&raw)
// 	})
// }

// func TestRawToParsed_vite_config_file_does_not_exist_panics(t *testing.T) {
// 	root := t.TempDir()
// 	raw := minimal_server_only_raw(t, root)
// 	mk_dir(t, filepath.Join(root, "jspkgdir"))
// 	raw.Vite = RawVite{
// 		JSPackageManagerBaseCmd: "pnpm",
// 		JSPackageManagerCmdDir:  "jspkgdir",
// 		ViteConfigFile:          "nope.ts",
// 	}
// 	must_panic(t, "vite config file missing", func() {
// 		RawToParsed(&raw)
// 	})
// }

// func TestRawToParsed_vite_port_negative_panics(t *testing.T) {
// 	root := t.TempDir()
// 	raw := minimal_server_only_raw(t, root)
// 	mk_dir(t, filepath.Join(root, "jspkgdir"))
// 	raw.Vite = RawVite{
// 		JSPackageManagerBaseCmd: "pnpm",
// 		JSPackageManagerCmdDir:  "jspkgdir",
// 		DefaultPort:             -1,
// 	}
// 	must_panic(t, "negative port", func() {
// 		RawToParsed(&raw)
// 	})
// }

// func TestRawToParsed_vite_port_too_high_panics(t *testing.T) {
// 	root := t.TempDir()
// 	raw := minimal_server_only_raw(t, root)
// 	mk_dir(t, filepath.Join(root, "jspkgdir"))
// 	raw.Vite = RawVite{
// 		JSPackageManagerBaseCmd: "pnpm",
// 		JSPackageManagerCmdDir:  "jspkgdir",
// 		DefaultPort:             99999,
// 	}
// 	must_panic(t, "port too high", func() {
// 		RawToParsed(&raw)
// 	})
// }

// func TestRawToParsed_watch_absolute_pattern_panics(t *testing.T) {
// 	root := t.TempDir()
// 	raw := minimal_server_only_raw(t, root)
// 	raw.Watch.Include = []RawIncludeEntry{
// 		{Pattern: "/absolute/**"},
// 	}
// 	must_panic(t, "absolute watch pattern", func() {
// 		RawToParsed(&raw)
// 	})
// }

// func TestRawToParsed_watch_invalid_glob_panics(t *testing.T) {
// 	root := t.TempDir()
// 	raw := minimal_server_only_raw(t, root)
// 	raw.Watch.Include = []RawIncludeEntry{
// 		{Pattern: "[invalid"},
// 	}
// 	must_panic(t, "invalid watch glob", func() {
// 		RawToParsed(&raw)
// 	})
// }

// func TestRawToParsed_watch_exclude_absolute_panics(t *testing.T) {
// 	root := t.TempDir()
// 	raw := minimal_server_only_raw(t, root)
// 	raw.Watch.Exclude = []string{"/absolute/**"}
// 	must_panic(t, "absolute watch exclude", func() {
// 		RawToParsed(&raw)
// 	})
// }

// func TestRawToParsed_watch_exclude_invalid_glob_panics(t *testing.T) {
// 	root := t.TempDir()
// 	raw := minimal_server_only_raw(t, root)
// 	raw.Watch.Exclude = []string{"[invalid"}
// 	must_panic(t, "invalid watch exclude glob", func() {
// 		RawToParsed(&raw)
// 	})
// }

// func TestRawToParsed_hook_empty_cmd_panics(t *testing.T) {
// 	root := t.TempDir()
// 	raw := minimal_server_only_raw(t, root)
// 	raw.Watch.Include = []RawIncludeEntry{
// 		{
// 			Pattern: "**/*.go",
// 			OnChangeHooks: []RawOnChangeHook{
// 				{Cmd: "  ", Timing: "pre"},
// 			},
// 		},
// 	}
// 	must_panic(t, "empty hook cmd", func() {
// 		RawToParsed(&raw)
// 	})
// }

// func TestRawToParsed_hook_bad_timing_panics(t *testing.T) {
// 	root := t.TempDir()
// 	raw := minimal_server_only_raw(t, root)
// 	raw.Watch.Include = []RawIncludeEntry{
// 		{
// 			Pattern: "**/*.go",
// 			OnChangeHooks: []RawOnChangeHook{
// 				{Cmd: "echo hi", Timing: "banana"},
// 			},
// 		},
// 	}
// 	must_panic(t, "bad timing", func() {
// 		RawToParsed(&raw)
// 	})
// }

// func TestRawToParsed_hook_timing_case_insensitive(t *testing.T) {
// 	root := t.TempDir()
// 	raw := minimal_server_only_raw(t, root)
// 	raw.Watch.Include = []RawIncludeEntry{
// 		{
// 			Pattern: "**/*.go",
// 			OnChangeHooks: []RawOnChangeHook{
// 				{Cmd: "echo hi", Timing: "PRE"},
// 				{Cmd: "echo hi", Timing: "Concurrent"},
// 				{Cmd: "echo hi", Timing: "CONCURRENT-NO-WAIT"},
// 				{Cmd: "echo hi", Timing: "Post"},
// 			},
// 		},
// 	}
// 	must_not_panic(t, "case insensitive timing", func() {
// 		parsed := RawToParsed(&raw)
// 		if parsed.Watch.Include[0].OnChangeHooks[0].Timing != OnChangeHookTimingPre {
// 			t.Fatal("PRE should normalize to pre")
// 		}
// 		if parsed.Watch.Include[0].OnChangeHooks[1].Timing != OnChangeHookTimingConcurrent {
// 			t.Fatal("Concurrent should normalize to concurrent")
// 		}
// 		if parsed.Watch.Include[0].OnChangeHooks[2].Timing != OnChangeHookTimingConcurrentNoWait {
// 			t.Fatal("CONCURRENT-NO-WAIT should normalize to concurrent-no-wait")
// 		}
// 		if parsed.Watch.Include[0].OnChangeHooks[3].Timing != OnChangeHookTimingPost {
// 			t.Fatal("Post should normalize to post")
// 		}
// 	})
// }

// func TestRawToParsed_hook_exclude_absolute_panics(t *testing.T) {
// 	root := t.TempDir()
// 	raw := minimal_server_only_raw(t, root)
// 	raw.Watch.Include = []RawIncludeEntry{
// 		{
// 			Pattern: "**/*.go",
// 			OnChangeHooks: []RawOnChangeHook{
// 				{Cmd: "echo hi", Timing: "pre", Exclude: []string{"/abs/**"}},
// 			},
// 		},
// 	}
// 	must_panic(t, "absolute hook exclude", func() {
// 		RawToParsed(&raw)
// 	})
// }

// func TestRawToParsed_hook_exclude_invalid_glob_panics(t *testing.T) {
// 	root := t.TempDir()
// 	raw := minimal_server_only_raw(t, root)
// 	raw.Watch.Include = []RawIncludeEntry{
// 		{
// 			Pattern: "**/*.go",
// 			OnChangeHooks: []RawOnChangeHook{
// 				{Cmd: "echo hi", Timing: "pre", Exclude: []string{"[bad"}},
// 			},
// 		},
// 	}
// 	must_panic(t, "invalid hook exclude glob", func() {
// 		RawToParsed(&raw)
// 	})
// }

// func TestRawToParsed_globs_resolved_against_resolve_root(t *testing.T) {
// 	root := t.TempDir()
// 	sub_dir := filepath.Join(root, "sub")
// 	mk_dir(t, sub_dir)
// 	touch_file(t, filepath.Join(root, "main.go"))

// 	cfg_file := filepath.Join(sub_dir, "wave.json")
// 	touch_file(t, cfg_file)

// 	raw := Raw{
// 		ConfigPath: strict.CWDRelPath(rel_from_cwd(t, cfg_file)),
// 		Core: RawCore{
// 			ResolveRoot:    "../",
// 			MainAppEntry:   "main.go",
// 			ServerOnlyMode: true,
// 		},
// 		Watch: RawWatch{
// 			Include: []RawIncludeEntry{
// 				{
// 					Pattern: "src/**/*.go",
// 					OnChangeHooks: []RawOnChangeHook{
// 						{
// 							Cmd:     "echo hi",
// 							Timing:  "pre",
// 							Exclude: []string{"vendor/**"},
// 						},
// 					},
// 				},
// 			},
// 			Exclude: []string{"tmp/**"},
// 		},
// 	}

// 	parsed := RawToParsed(&raw)
// 	rel_root := rel_from_cwd(t, root)

// 	// all globs should be prefixed with the resolve root (root, not sub)
// 	expect_path(t, "Include[0].Pattern",
// 		parsed.Watch.Include[0].Pattern, rel_root, "src/**/*.go")
// 	expect_path(
// 		t,
// 		"Include[0].OnChangeHooks[0].Exclude[0]",
// 		parsed.Watch.Include[0].OnChangeHooks[0].Exclude[0],
// 		rel_root,
// 		"vendor/**",
// 	)
// 	expect_path(t, "Exclude[0]",
// 		parsed.Watch.Exclude[0], rel_root, "tmp/**")
// }

// func TestRawToParsed_empty_watch(t *testing.T) {
// 	root := t.TempDir()
// 	raw := minimal_server_only_raw(t, root)
// 	must_not_panic(t, "empty watch", func() {
// 		parsed := RawToParsed(&raw)
// 		if len(parsed.Watch.Include) != 0 {
// 			t.Fatalf(
// 				"Watch.Include should be empty, got %d",
// 				len(parsed.Watch.Include),
// 			)
// 		}
// 		if len(parsed.Watch.Exclude) != 0 {
// 			t.Fatalf(
// 				"Watch.Exclude should be empty, got %d",
// 				len(parsed.Watch.Exclude),
// 			)
// 		}
// 	})
// }

// func TestRawToParsed_messy_whitespace_everywhere(t *testing.T) {
// 	root := t.TempDir()
// 	full_project_fs(t, root)
// 	raw := full_project_config(t, root)

// 	raw.Core.DevBuildHook = "   make dev   "
// 	raw.Core.ProdBuildHook = "\t make prod \t"
// 	raw.Watch.HealthcheckEndpoint = "  healthz  "

// 	must_not_panic(t, "messy whitespace", func() {
// 		parsed := RawToParsed(&raw)
// 		if parsed.Core.DevBuildHook != "make dev" {
// 			t.Fatalf("DevBuildHook: got %q", parsed.Core.DevBuildHook)
// 		}
// 		if parsed.Core.ProdBuildHook != "make prod" {
// 			t.Fatalf("ProdBuildHook: got %q", parsed.Core.ProdBuildHook)
// 		}
// 		if parsed.Watch.HealthcheckEndpoint != "/healthz" {
// 			t.Fatalf(
// 				"HealthcheckEndpoint: got %q",
// 				parsed.Watch.HealthcheckEndpoint,
// 			)
// 		}
// 	})
// }

// func TestRawToParsed_vite_optional_config_file_omitted(t *testing.T) {
// 	root := t.TempDir()
// 	raw := minimal_server_only_raw(t, root)
// 	mk_dir(t, filepath.Join(root, "jspkgdir"))
// 	raw.Vite = RawVite{
// 		JSPackageManagerBaseCmd: "pnpm",
// 		JSPackageManagerCmdDir:  "jspkgdir",
// 		ViteConfigFile:          "",
// 	}
// 	must_not_panic(t, "omitted vite config file", func() {
// 		parsed := RawToParsed(&raw)
// 		if parsed.Vite.ViteConfigFile != "" {
// 			t.Fatalf(
// 				"ViteConfigFile should be empty, got %q",
// 				parsed.Vite.ViteConfigFile,
// 			)
// 		}
// 	})
// }

// func TestRawToParsed_watch_include_no_hooks(t *testing.T) {
// 	root := t.TempDir()
// 	raw := minimal_server_only_raw(t, root)
// 	raw.Watch.Include = []RawIncludeEntry{
// 		{
// 			Pattern:           "**/*.go",
// 			RecompileGoBinary: true,
// 		},
// 	}
// 	must_not_panic(t, "include with no hooks", func() {
// 		parsed := RawToParsed(&raw)
// 		if len(parsed.Watch.Include[0].OnChangeHooks) != 0 {
// 			t.Fatal("expected no hooks")
// 		}
// 	})
// }

// func TestRawToParsed_empty_resolve_root_defaults_to_config_dir(t *testing.T) {
// 	root := t.TempDir()
// 	touch_file(t, filepath.Join(root, "main.go"))
// 	cfg_file := filepath.Join(root, "wave.json")
// 	touch_file(t, cfg_file)

// 	raw := Raw{
// 		ConfigPath: strict.CWDRelPath(rel_from_cwd(t, cfg_file)),
// 		Core: RawCore{
// 			ResolveRoot:    "",
// 			MainAppEntry:   "main.go",
// 			ServerOnlyMode: true,
// 		},
// 	}
// 	must_not_panic(t, "empty resolve root", func() {
// 		parsed := RawToParsed(&raw)
// 		expect_path(
// 			t,
// 			"MainAppEntry",
// 			parsed.Core.MainAppEntry,
// 			rel_from_cwd(t, root),
// 			"main.go",
// 		)
// 	})
// }

// func TestRawToParsed_absolute_resolve_root_panics(t *testing.T) {
// 	root := t.TempDir()
// 	cfg_file := filepath.Join(root, "wave.json")
// 	touch_file(t, cfg_file)

// 	raw := Raw{
// 		ConfigPath: strict.CWDRelPath(rel_from_cwd(t, cfg_file)),
// 		Core: RawCore{
// 			ResolveRoot:    "/absolute/root",
// 			MainAppEntry:   "main.go",
// 			ServerOnlyMode: true,
// 		},
// 	}
// 	must_panic(t, "absolute resolve root", func() {
// 		RawToParsed(&raw)
// 	})
// }

// func TestRawToParsed_absolute_static_private_dir_panics(t *testing.T) {
// 	root := t.TempDir()
// 	full_project_fs(t, root)
// 	raw := full_project_config(t, root)
// 	raw.Core.StaticAssetDirs = RawStaticAssetDirs{
// 		Private: "/absolute/private",
// 		Public:  "public",
// 	}
// 	must_panic(t, "absolute static private dir", func() {
// 		RawToParsed(&raw)
// 	})
// }

// func TestRawToParsed_absolute_static_public_dir_panics(t *testing.T) {
// 	root := t.TempDir()
// 	full_project_fs(t, root)
// 	raw := full_project_config(t, root)
// 	raw.Core.StaticAssetDirs = RawStaticAssetDirs{
// 		Private: "private",
// 		Public:  "/absolute/public",
// 	}
// 	must_panic(t, "absolute static public dir", func() {
// 		RawToParsed(&raw)
// 	})
// }

// func TestRawToParsed_absolute_critical_css_panics(t *testing.T) {
// 	root := t.TempDir()
// 	full_project_fs(t, root)
// 	raw := full_project_config(t, root)
// 	raw.Core.CSSEntryFiles = RawCSSEntryFiles{
// 		Critical:    "/absolute/critical.css",
// 		NonCritical: "noncritical.css",
// 	}
// 	must_panic(t, "absolute critical css", func() {
// 		RawToParsed(&raw)
// 	})
// }

// func TestRawToParsed_absolute_non_critical_css_panics(t *testing.T) {
// 	root := t.TempDir()
// 	full_project_fs(t, root)
// 	raw := full_project_config(t, root)
// 	raw.Core.CSSEntryFiles = RawCSSEntryFiles{
// 		Critical:    "critical.css",
// 		NonCritical: "/absolute/noncritical.css",
// 	}
// 	must_panic(t, "absolute non-critical css", func() {
// 		RawToParsed(&raw)
// 	})
// }

// func TestRawToParsed_absolute_js_pkg_mgr_cmd_dir_panics(t *testing.T) {
// 	root := t.TempDir()
// 	raw := minimal_server_only_raw(t, root)
// 	raw.Vite = RawVite{
// 		JSPackageManagerBaseCmd: "pnpm",
// 		JSPackageManagerCmdDir:  "/absolute/jspkgdir",
// 	}
// 	must_panic(t, "absolute js pkg mgr cmd dir", func() {
// 		RawToParsed(&raw)
// 	})
// }

// func TestRawToParsed_absolute_vite_config_file_panics(t *testing.T) {
// 	root := t.TempDir()
// 	raw := minimal_server_only_raw(t, root)
// 	mk_dir(t, filepath.Join(root, "jspkgdir"))
// 	raw.Vite = RawVite{
// 		JSPackageManagerBaseCmd: "pnpm",
// 		JSPackageManagerCmdDir:  "jspkgdir",
// 		ViteConfigFile:          "/absolute/vite.config.ts",
// 	}
// 	must_panic(t, "absolute vite config file", func() {
// 		RawToParsed(&raw)
// 	})
// }

// func TestRawToParsed_hook_empty_timing_defaults_to_pre(t *testing.T) {
// 	root := t.TempDir()
// 	raw := minimal_server_only_raw(t, root)
// 	raw.Watch.Include = []RawIncludeEntry{
// 		{
// 			Pattern: "**/*.go",
// 			OnChangeHooks: []RawOnChangeHook{
// 				{Cmd: "echo hi", Timing: ""},
// 			},
// 		},
// 	}
// 	must_not_panic(t, "empty timing defaults to pre", func() {
// 		parsed := RawToParsed(&raw)
// 		if parsed.Watch.Include[0].OnChangeHooks[0].Timing != OnChangeHookTimingPre {
// 			t.Fatalf(
// 				"expected pre, got %q",
// 				parsed.Watch.Include[0].OnChangeHooks[0].Timing,
// 			)
// 		}
// 	})
// }

// func TestRawToParsed_hook_multiple_excludes(t *testing.T) {
// 	root := t.TempDir()
// 	raw := minimal_server_only_raw(t, root)
// 	raw.Watch.Include = []RawIncludeEntry{
// 		{
// 			Pattern: "**/*.go",
// 			OnChangeHooks: []RawOnChangeHook{
// 				{
// 					Cmd:    "echo hi",
// 					Timing: "pre",
// 					Exclude: []string{
// 						"vendor/**",
// 						"tmp/cache/**",
// 						"**/*.generated.go",
// 						"internal/testdata/**/*.go",
// 					},
// 				},
// 			},
// 		},
// 	}
// 	rel_root := rel_from_cwd(t, root)
// 	must_not_panic(t, "multiple hook excludes", func() {
// 		parsed := RawToParsed(&raw)
// 		excludes := parsed.Watch.Include[0].OnChangeHooks[0].Exclude
// 		if len(excludes) != 4 {
// 			t.Fatalf("expected 4 excludes, got %d", len(excludes))
// 		}
// 		expect_path(t, "exclude[0]", excludes[0], rel_root, "vendor/**")
// 		expect_path(t, "exclude[1]", excludes[1], rel_root, "tmp/cache/**")
// 		expect_path(t, "exclude[2]", excludes[2], rel_root, "**/*.generated.go")
// 		expect_path(
// 			t,
// 			"exclude[3]",
// 			excludes[3],
// 			rel_root,
// 			"internal/testdata/**/*.go",
// 		)
// 	})
// }

// func TestRawToParsed_watch_include_dir_and_file_patterns(t *testing.T) {
// 	root := t.TempDir()
// 	raw := minimal_server_only_raw(t, root)
// 	raw.Watch.Include = []RawIncludeEntry{
// 		{Pattern: "src/**/*.go", RecompileGoBinary: true},
// 		{Pattern: "assets/**", TreatAsNonGo: true},
// 		{Pattern: "config/*.json", RunOnChangeOnly: true},
// 		{Pattern: "templates/**/*.html", SkipRebuildingNotification: true},
// 	}
// 	rel_root := rel_from_cwd(t, root)
// 	must_not_panic(t, "dir and file include patterns", func() {
// 		parsed := RawToParsed(&raw)
// 		if len(parsed.Watch.Include) != 4 {
// 			t.Fatalf("expected 4 includes, got %d", len(parsed.Watch.Include))
// 		}
// 		expect_path(
// 			t,
// 			"include[0]",
// 			parsed.Watch.Include[0].Pattern,
// 			rel_root,
// 			"src/**/*.go",
// 		)
// 		expect_path(
// 			t,
// 			"include[1]",
// 			parsed.Watch.Include[1].Pattern,
// 			rel_root,
// 			"assets/**",
// 		)
// 		expect_path(
// 			t,
// 			"include[2]",
// 			parsed.Watch.Include[2].Pattern,
// 			rel_root,
// 			"config/*.json",
// 		)
// 		expect_path(
// 			t,
// 			"include[3]",
// 			parsed.Watch.Include[3].Pattern,
// 			rel_root,
// 			"templates/**/*.html",
// 		)
// 	})
// }

// func TestRawToParsed_watch_exclude_dir_and_file_patterns(t *testing.T) {
// 	root := t.TempDir()
// 	raw := minimal_server_only_raw(t, root)
// 	raw.Watch.Exclude = []string{
// 		"vendor/**",
// 		"node_modules/**",
// 		"**/*.test.go",
// 		"dist/**/*.map",
// 	}
// 	rel_root := rel_from_cwd(t, root)
// 	must_not_panic(t, "dir and file exclude patterns", func() {
// 		parsed := RawToParsed(&raw)
// 		if len(parsed.Watch.Exclude) != 4 {
// 			t.Fatalf("expected 4 excludes, got %d", len(parsed.Watch.Exclude))
// 		}
// 		expect_path(
// 			t,
// 			"exclude[0]",
// 			parsed.Watch.Exclude[0],
// 			rel_root,
// 			"vendor/**",
// 		)
// 		expect_path(
// 			t,
// 			"exclude[1]",
// 			parsed.Watch.Exclude[1],
// 			rel_root,
// 			"node_modules/**",
// 		)
// 		expect_path(
// 			t,
// 			"exclude[2]",
// 			parsed.Watch.Exclude[2],
// 			rel_root,
// 			"**/*.test.go",
// 		)
// 		expect_path(
// 			t,
// 			"exclude[3]",
// 			parsed.Watch.Exclude[3],
// 			rel_root,
// 			"dist/**/*.map",
// 		)
// 	})
// }

// func TestRawToParsed_multiple_valid_ports(t *testing.T) {
// 	root := t.TempDir()
// 	raw := minimal_server_only_raw(t, root)
// 	mk_dir(t, filepath.Join(root, "jspkgdir"))

// 	for _, port := range []int{1, 80, 443, 3000, 8080, 65535} {
// 		raw.Vite = RawVite{
// 			JSPackageManagerBaseCmd: "pnpm",
// 			JSPackageManagerCmdDir:  "jspkgdir",
// 			DefaultPort:             port,
// 		}
// 		must_not_panic(t, fmt.Sprintf("port %d", port), func() {
// 			parsed := RawToParsed(&raw)
// 			if int(parsed.Vite.DefaultPort) != port {
// 				t.Fatalf("port %d: got %d", port, parsed.Vite.DefaultPort)
// 			}
// 		})
// 	}
// }
