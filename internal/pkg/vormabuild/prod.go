package vormabuild

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/vormadev/vorma/internal/pkg/vormarun"
)

func (cfg vorma_cfg) prod_tmp_vite_manifest_out() string {
	return filepath.Join(cfg.pub_out(), vormarun.Prod_Tmp_Vite_Manifest_Filename)
}

func (cfg vorma_cfg) remove_prod_tmp_vite_manifest() error {
	if err := os.Remove(cfg.prod_tmp_vite_manifest_out()); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return nil
}

func (cfg vorma_cfg) run_vite_prod_build_cmd(
	ctx context.Context,
	internal_dev_server_port int,
) (*exec.Cmd, error) {
	// Needs to be made relative to js_package_manager_dir given that
	// that is where the vite command is being run from
	out_dir, err := filepath.Rel(cfg.js_package_manager_dir(), cfg.pub_out())
	if err != nil {
		panic("failed to get relative path for Vite outDir: " + err.Error())
	}

	args := append(cfg.js_package_manager_cmd_base(),
		"vite", "build",
		"--outDir", out_dir,
		"--assetsDir", ".",
		"--manifest", vormarun.Prod_Tmp_Vite_Manifest_Filename,
		"--emptyOutDir", "false",
	)
	cfg_file := cfg.vite_config_file()
	if cfg_file != "" {
		args = append(args, "--config", cfg_file)
	}

	cmd := new_cmd(ctx, args[0], args[1:]...)
	cmd.Dir = cfg.js_package_manager_dir()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(cmd.Env, env_item_int(
		vite_plugin_go_port_env_key,
		internal_dev_server_port,
	))

	return cmd, nil
}
