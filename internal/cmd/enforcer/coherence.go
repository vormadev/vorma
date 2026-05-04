package main

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
)

var known_go_modules = []string{
	"go.mod",
	"internal/apps/docs/go.mod",
	"internal/framework_tests/go.mod",
	"internal/matcher_tests/go.mod",
}

var known_package_manifests = []string{
	"package.json",
	"internal/apps/docs/package.json",
	"internal/framework_tests/package.json",
	"internal/matcher_tests/package.json",
	"internal/pkg/npm/package.json",
	"internal/pkg/npm/vorma/create/package.json",
}

var known_ts_projects = []string{
	"internal/apps/docs/tsconfig.json",
	"internal/framework_tests/tsconfig.json",
	"internal/matcher_tests/tsconfig.json",
	"internal/pkg/npm/kit/tsconfig.json",
	"internal/pkg/npm/vorma/core/tsconfig.json",
	"internal/pkg/npm/vorma/create/tsconfig.json",
	"internal/pkg/npm/vorma/tests/tsconfig.json",
	"internal/pkg/npm/vorma/tsx/preact/tsconfig.json",
	"internal/pkg/npm/vorma/tsx/react/tsconfig.json",
	"internal/pkg/npm/vorma/tsx/solid/tsconfig.json",
	"internal/pkg/npm/vorma/vite/tsconfig.json",
}

type repo_path_list []string

func (app enforcer_app) check_repo_shape() error {
	if app.DryRun {
		fmt.Println("==> verify repo enforcement shape")
		fmt.Println("<== verify repo enforcement shape passed (dry run)")
		return nil
	}
	if err := app.check_known_repo_files("Go module", "go.mod", known_go_modules); err != nil {
		return err
	}
	if err := app.check_known_repo_files(
		"package manifest",
		"package.json",
		known_package_manifests,
	); err != nil {
		return err
	}
	return app.check_known_repo_files("TypeScript project", "tsconfig.json", known_ts_projects)
}

func (app enforcer_app) check_known_repo_files(
	label string,
	file_name string,
	known []string,
) error {
	actual, err := app.find_repo_files_named(file_name)
	if err != nil {
		return err
	}

	unknown := []string{}
	for _, path := range actual {
		if !slices.Contains(known, path) {
			unknown = append(unknown, path)
		}
	}
	missing := []string{}
	for _, path := range known {
		if !slices.Contains(actual, path) {
			missing = append(missing, path)
		}
	}
	if len(unknown) == 0 && len(missing) == 0 {
		return nil
	}

	msg := strings.Builder{}
	fmt.Fprintf(&msg, "%s classification is stale", label)
	if len(unknown) > 0 {
		fmt.Fprintf(&msg, "\nunknown %s file(s):\n%s", label, repo_path_list(unknown).format())
	}
	if len(missing) > 0 {
		fmt.Fprintf(
			&msg,
			"\nmissing known %s file(s):\n%s",
			label,
			repo_path_list(missing).format(),
		)
	}
	return fmt.Errorf("%s", msg.String())
}

func (app enforcer_app) find_repo_files_named(file_name string) ([]string, error) {
	paths := []string{}
	err := filepath.WalkDir(app.Root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && should_skip_repo_shape_dir(entry.Name()) {
			return filepath.SkipDir
		}
		if entry.IsDir() || entry.Name() != file_name {
			return nil
		}

		rel_path, err := filepath.Rel(app.Root, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(rel_path))
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(paths)
	return paths, nil
}

func should_skip_repo_shape_dir(name string) bool {
	if strings.HasPrefix(name, ".dist.") {
		return true
	}
	return slices.Contains([]string{
		".bombadil",
		".git",
		".pnpm-store",
		"__.local",
		"__LLM_CONCAT.local",
		"node_modules",
	}, name)
}

func (paths repo_path_list) format() string {
	msg := strings.Builder{}
	for _, path := range paths {
		fmt.Fprintf(&msg, "  - %s\n", path)
	}
	return strings.TrimRight(msg.String(), "\n")
}
