package wavebuild

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/kit/strict"
)

func TestConfigPathToValidatedConfig_AllowsRootDirDot(t *testing.T) {
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

	cfg_path := temp_dir.Join("wave.config.json")
	if err := os.WriteFile(
		cfg_path.Str(),
		[]byte(`{
	"RootDir": ".",
	"Core": {
		"BinaryName": "main"
	}
}`),
		0644,
	); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := config_path_to_validated_config(cfg_path)
	if err != nil {
		t.Fatalf("config_path_to_validated_config: %v", err)
	}
	if got, want := cfg.root_dir, temp_dir; got != want {
		t.Fatalf("root_dir = %q, want %q", got, want)
	}
}

func TestConfigPathToValidatedConfig_RejectsPublicDirOverlappingWaveout(
	t *testing.T,
) {
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

	cfg_path := temp_dir.Join("wave.config.json")
	if err := os.WriteFile(
		cfg_path.Str(),
		[]byte(`{
	"RootDir": ".",
	"Core": {
		"BinaryName": "main",
		"StaticAssetDirs": {
			"Public": "."
		}
	}
}`),
		0644,
	); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err = config_path_to_validated_config(cfg_path)
	if err == nil {
		t.Fatal("config_path_to_validated_config unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), ".waveout") {
		t.Fatalf("config_path_to_validated_config error = %q", err)
	}
}
