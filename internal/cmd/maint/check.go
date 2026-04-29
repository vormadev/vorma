package main

func (app maint_app) install_js() error {
	return app.run_work(work_install_js)
}

func (app maint_app) clean_js() error {
	if err := app.confirm("Remove every node_modules directory?"); err != nil {
		return err
	}
	return app.run_steps([]command_step{
		{name: "remove root node_modules", command: "rm", args: []string{"-rf", "node_modules"}},
		{
			name:    "remove nested node_modules",
			command: "find",
			args: []string{
				".",
				"-path",
				"*/node_modules",
				"-type",
				"d",
				"-prune",
				"-exec",
				"rm",
				"-rf",
				"{}",
				"+",
			},
		},
	})
}

func (app maint_app) fmt_ts() error {
	return app.run_work(work_fmt_ts)
}

func (app maint_app) lint_ts() error {
	return app.run_work(work_lint_ts)
}

func (app maint_app) typecheck_ts() error {
	return app.run_work(work_typecheck_ts)
}

func (app maint_app) typecheck_ts_other() error {
	return app.run_work(work_typecheck_ts_other)
}

func (app maint_app) typecheck_ts_framework() error {
	return app.run_work(work_typecheck_ts_framework)
}

func (app maint_app) test_ts() error {
	return app.run_work(work_test_ts)
}

func (app maint_app) test_ts_other() error {
	return app.run_work(work_test_ts_other)
}

func (app maint_app) test_ts_framework() error {
	return app.run_work(work_test_ts_framework)
}

func (app maint_app) build_ts() error {
	return app.run_work(work_build_ts)
}

func (app maint_app) test_go() error {
	return app.run_work(work_test_go)
}

func (app maint_app) test_go_other() error {
	return app.run_work(work_test_go_other)
}

func (app maint_app) test_root_go() error {
	return app.run_work(work_test_root_go)
}

func (app maint_app) test_internal_go() error {
	return app.run_work(work_test_internal_go)
}

func (app maint_app) test_kit() error {
	return app.run_work(work_test_kit)
}

func (app maint_app) test_lab() error {
	return app.run_work(work_test_lab)
}

func (app maint_app) test_docs() error {
	return app.run_work(work_test_docs)
}

func (app maint_app) test_go_framework() error {
	return app.run_work(work_test_go_framework)
}

func (app maint_app) gate() error {
	return app.run_work(work_gate)
}
