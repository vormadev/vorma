package main

func (app maint_app) docs_dev() error {
	return app.run_step(
		command_step{
			name:    "run docs development server",
			dir:     docs_dir,
			command: "pnpm",
			args:    []string{"dev"},
		},
	)
}

func (app maint_app) build_docs() error {
	return app.run_step(
		command_step{
			name:    "build docs site",
			dir:     docs_dir,
			command: "pnpm",
			args:    []string{"build"},
		},
	)
}
