package vormabuild

// Route definition file contract:
//
// Route definition files are matched by Vorma.ClientRouteDefinitionPatterns.
// They are regular TypeScript/JavaScript files that import `route` from
// "vorma/buildtime" and call it to declare routes.
//
// At build time, these files are bundled with esbuild and executed with
// Node to extract route declarations. This means you can use any
// language feature — helpers, loops, conditionals, imports from other
// files — as long as the route() calls ultimately execute at module
// evaluation time.
//
// The one hard rule: dynamic import() anywhere in the dependency tree
// of route definition files must only be used for route module
// references passed to route(). This is because the build bundles the
// entire tree and transforms all import() calls to extract module paths.
//
// Example:
//
//   import { route } from "vorma/buildtime";
//
//   const shared = import("./components/shared.tsx");
//
//   route("/", import("./components/root.tsx"), "Root");
//   route("/about", shared, "About");
//   route("/whatever-else", shared, "WhateverElse");
//   route("/users/:id", import("./components/user.tsx"), "User", "UserError");

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/vormadev/vorma/kit/set"
	"github.com/vormadev/vorma/kit/strict"
)

type discovered_route struct {
	pattern          string
	module_path      strict.CWDRelPath
	export_key       string
	error_export_key string
}

// route_discovery_output matches the JSON shape logged by the
// buildtime route() function. Uses PascalCase keys so Go's
// json.Unmarshal matches struct fields without tags.
type route_discovery_output struct {
	Pattern        string
	Module         string
	ExportKey      string
	ErrorExportKey string
}

func discover_routes(
	patterns []strict.CWDRelPath,
) ([]discovered_route, error) {
	// expand glob patterns to concrete files
	file_set := &set.Set[string]{}
	for _, pattern := range patterns {
		matched, err := doublestar.FilepathGlob(pattern.Str())
		if err != nil {
			return nil, fmt.Errorf("expanding %q: %w", pattern, err)
		}
		for _, m := range matched {
			info, err := os.Stat(m)
			if err != nil {
				return nil, fmt.Errorf("stat %q: %w", m, err)
			}
			if !info.IsDir() {
				file_set.Add(filepath.Clean(m))
			}
		}
	}
	files := file_set.Slice()
	sort.Strings(files)
	if len(files) == 0 {
		return nil, fmt.Errorf("no route definition files matched")
	}

	// build virtual entry that imports all route files
	var entry strings.Builder
	for _, f := range files {
		abs, err := filepath.Abs(f)
		if err != nil {
			return nil, fmt.Errorf("abs path for %q: %w", f, err)
		}
		fmt.Fprintf(&entry, "import %q;\n", filepath.ToSlash(abs))
	}

	// bundle with esbuild — inlines all static imports (helpers,
	// vorma/buildtime), externalizes dynamic imports with resolved
	// absolute paths via plugin
	build_result := esbuild.Build(esbuild.BuildOptions{
		Stdin: &esbuild.StdinOptions{
			Contents:   entry.String(),
			ResolveDir: ".",
			Loader:     esbuild.LoaderTS,
		},
		Bundle:            true,
		Write:             false,
		Format:            esbuild.FormatESModule,
		Platform:          esbuild.PlatformNode,
		MinifyWhitespace:  true,
		MinifySyntax:      true,
		MinifyIdentifiers: false,
		Target:            esbuild.ES2020,
		LogLevel:          esbuild.LogLevelSilent,
		Plugins: []esbuild.Plugin{{
			Name: "resolve_dynamic_imports",
			Setup: func(build esbuild.PluginBuild) {
				build.OnResolve(
					esbuild.OnResolveOptions{Filter: ".*"},
					func(args esbuild.OnResolveArgs) (esbuild.OnResolveResult, error) {
						if args.Kind != esbuild.ResolveJSDynamicImport {
							return esbuild.OnResolveResult{}, nil
						}
						resolved := filepath.Clean(
							filepath.Join(args.ResolveDir, args.Path),
						)
						return esbuild.OnResolveResult{
							Path:     filepath.ToSlash(resolved),
							External: true,
						}, nil
					},
				)
			},
		}},
	})
	if len(build_result.Errors) > 0 {
		return nil, fmt.Errorf("esbuild: %s", build_result.Errors[0].Text)
	}
	if len(build_result.OutputFiles) == 0 {
		return nil, fmt.Errorf("esbuild produced no output")
	}

	// replace dynamic import calls with plain string expressions:
	//   import("/absolute/path/module.tsx")  →  ("/absolute/path/module.tsx")
	bundled := string(build_result.OutputFiles[0].Contents)
	bundled = strings.ReplaceAll(bundled, `import("`, `("`)
	bundled = strings.ReplaceAll(bundled, `import('`, `('`)

	// write to temp file and execute with node
	tmp, err := os.CreateTemp("", "vorma2-routes-*.mjs")
	if err != nil {
		return nil, fmt.Errorf("creating temp file: %w", err)
	}
	tmp_path := tmp.Name()
	defer os.Remove(tmp_path)

	if _, err := tmp.WriteString(bundled); err != nil {
		tmp.Close()
		return nil, fmt.Errorf("writing temp file: %w", err)
	}
	tmp.Close()

	cmd := exec.Command("node", tmp_path)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf(
			"node failed: %s\n%s", err, strings.TrimSpace(stderr.String()),
		)
	}

	// parse JSON lines from stdout
	var routes []discovered_route
	seen := &set.Set[string]{}

	for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var out route_discovery_output
		if err := json.Unmarshal([]byte(line), &out); err != nil {
			return nil, fmt.Errorf(
				"parsing route output: %w\nline: %s",
				err,
				line,
			)
		}
		if seen.Has(out.Pattern) {
			return nil, fmt.Errorf("duplicate route pattern %q", out.Pattern)
		}
		seen.Add(out.Pattern)

		// module path comes back absolute from our esbuild plugin —
		// convert to CWD-relative
		rel_module, err := filepath.Rel(".", out.Module)
		if err != nil {
			return nil, fmt.Errorf(
				"route(%q): cannot relativize module %q: %w",
				out.Pattern, out.Module, err,
			)
		}
		module_path := strict.MustNormalizeCWDRelPath(rel_module)

		if _, err := os.Stat(module_path.Str()); err != nil {
			return nil, fmt.Errorf(
				"route(%q): module %q does not exist",
				out.Pattern, module_path,
			)
		}

		routes = append(routes, discovered_route{
			pattern:          out.Pattern,
			module_path:      module_path,
			export_key:       out.ExportKey,
			error_export_key: out.ErrorExportKey,
		})
	}

	return routes, nil
}
