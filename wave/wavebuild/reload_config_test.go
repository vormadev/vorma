package wavebuild

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/vormadev/vorma/kit/strict"
)

func TestReloadConfig_UsesPluginHooksDerivedFromCurrentParsedConfig(
	t *testing.T,
) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	temp_dir_abs := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	temp_dir_rel, err := filepath.Rel(cwd, temp_dir_abs)
	if err != nil {
		t.Fatalf("filepath.Rel: %v", err)
	}
	temp_dir := strict.MustNormalizeCWDRelPath(temp_dir_rel)
	project_dir := temp_dir.Join("app")

	if err := os.MkdirAll(project_dir.Str(), 0755); err != nil {
		t.Fatalf("mkdir app: %v", err)
	}
	if err := os.WriteFile(
		project_dir.Join("README.md").Str(),
		[]byte("test"),
		0644,
	); err != nil {
		t.Fatalf("write README.md: %v", err)
	}

	type plugin_config_json struct {
		HookName string
	}

	plugin := &Plugin{
		Name: "test plugin",
		LifecycleHooks: []LifecycleHook{
			{
				Name:                 "stale hook",
				WatchIncludePatterns: []strict.CWDRelPath{"README.md"},
			},
		},
	}
	plugin.Config = PluginConfig{
		JSONKey: "TestPlugin",
		ParseFunc: func(ctx *PluginConfigParseCtx) error {
			if ctx.RawJSON == nil {
				plugin.LifecycleHooks = nil
				return nil
			}

			var cfg plugin_config_json
			if err := json.Unmarshal(*ctx.RawJSON, &cfg); err != nil {
				return err
			}

			plugin.LifecycleHooks = []LifecycleHook{
				{
					Name:                 cfg.HookName,
					WatchIncludePatterns: []strict.CWDRelPath{"README.md"},
				},
			}
			return nil
		},
	}

	plugin_cfg, err := validate_plugin_config(plugin.Config)
	if err != nil {
		t.Fatalf("validate_plugin_config: %v", err)
	}

	cfg_path := project_dir.Join("wave.config.json")
	write_config := func(hook_name string) {
		t.Helper()
		json_bytes := []byte(`{
	"RootDir": ".",
	"Core": {
		"BinaryName": "main"
	},
	"TestPlugin": {
		"HookName": "` + hook_name + `"
	}
}`)
		if err := os.WriteFile(cfg_path.Str(), json_bytes, 0644); err != nil {
			t.Fatalf("write config: %v", err)
		}
	}

	s := &super_state{
		cfg_path:    cfg_path,
		logger:      logger,
		plugins:     []*Plugin{plugin},
		plugin_cfgs: []*validated_plugin_config{plugin_cfg},
	}

	write_config("first parsed hook")
	if err := s.reload_config(); err != nil {
		t.Fatalf("reload_config first: %v", err)
	}
	if len(s.all_hooks) != 1 {
		t.Fatalf("first reload all_hooks len = %d, want 1", len(s.all_hooks))
	}
	if got := s.all_hooks[0].name; got != "first parsed hook" {
		t.Fatalf(
			"first reload hook name = %q, want %q",
			got,
			"first parsed hook",
		)
	}

	write_config("second parsed hook")
	if err := s.reload_config(); err != nil {
		t.Fatalf("reload_config second: %v", err)
	}
	if len(s.all_hooks) != 1 {
		t.Fatalf("second reload all_hooks len = %d, want 1", len(s.all_hooks))
	}
	if got := s.all_hooks[0].name; got != "second parsed hook" {
		t.Fatalf(
			"second reload hook name = %q, want %q",
			got,
			"second parsed hook",
		)
	}
}
