package main

func (app maint_app) create_local_test() error {
	if err := app.install_js(); err != nil {
		return err
	}
	if err := app.build_ts(); err != nil {
		return err
	}
	if err := app.clean_js(); err != nil {
		return err
	}
	return app.run_steps([]command_step{
		{
			name:    "create local test directory",
			command: "mkdir",
			args:    []string{"-p", "test_create.local"},
		},
		{
			name:    "run create-vorma local test",
			dir:     "test_create.local",
			command: "node",
			args:    []string{"../internal/pkg/npm/vorma/create/.dist/main.js", "--local-test"},
		},
	})
}
