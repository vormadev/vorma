package main

import (
	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/kit/lab/repoconcat"
)

const out = "__.local/"

func main() {
	if err := fsutil.EnsureDir(out); err != nil {
		panic(err)
	}

	repoconcat.MustConcat(out+"new__backend_buildtime.txt", []string{
		"!**/*_test.go",

		"vormabuild/**/*.go",
		"internal/pkg/vormabuild/**/*.go",
		"internal/pkg/cssbundle/**/*.go",
		"internal/pkg/fswatcher/**/*.go",
		"internal/pkg/mailbox/**/*.go",
		"internal/pkg/staticproc/**/*.go",
		"internal/pkg/viteutil/**/*.go",
	})

	repoconcat.MustConcat(out+"new__backend_runtime.txt", []string{
		"!**/*_test.go",

		"vorma.go",
		"internal/pkg/vormarun/**/*.go",
		"internal/pkg/vormarun/refresh_script.js",
	})

	repoconcat.MustConcat(out+"new__frontend.txt", []string{
		"!**/*.test.*",
		"!tsconfig.json",
		"!internal/pkg/npm/.dist",
		"!internal/pkg/npm/vorma/tests/",
		"!internal/pkg/npm/vorma/core/___ccc_test_helpers.ts",

		"internal/pkg/npm/vorma/",
	})

	repoconcat.MustConcat(out+"new__frontend_tests.txt", []string{
		"internal/pkg/npm/vorma/tests/**/*.ts",
		"internal/pkg/npm/vorma/**/*.test.*",
	})
}
