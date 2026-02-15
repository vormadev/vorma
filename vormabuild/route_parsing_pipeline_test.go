package vormabuild

import (
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/vormadev/vorma/vormaruntime"
)

func TestResolveClientRouteDefinitionFiles(t *testing.T) {
	originalRouteDefinitionsFileResolutionDeps := routeDefinitionsFileResolutionDeps
	t.Cleanup(func() {
		routeDefinitionsFileResolutionDeps = originalRouteDefinitionsFileResolutionDeps
	})

	t.Run("returns required error when runtime is nil", func(t *testing.T) {
		_, err := resolveClientRouteDefinitionFiles(nil)
		if err == nil {
			t.Fatal("expected error when runtime is nil")
		}
		if !strings.Contains(err.Error(), "Vorma runtime is required") {
			t.Fatalf("error = %q, expected nil-runtime message", err)
		}
	})

	t.Run("returns required error when runtime config is nil", func(t *testing.T) {
		v := &vormaruntime.Vorma{
			Log: testLogger(),
		}

		_, err := resolveClientRouteDefinitionFiles(v)
		if err == nil {
			t.Fatal("expected error when runtime config is nil")
		}
		if !strings.Contains(err.Error(), "Vorma config is required") {
			t.Fatalf("error = %q, expected nil-config message", err)
		}
	})

	t.Run("returns required error when route definition patterns are missing", func(t *testing.T) {
		v := &vormaruntime.Vorma{
			Config: &vormaruntime.VormaConfig{},
			Log:    testLogger(),
		}

		_, err := resolveClientRouteDefinitionFiles(v)
		if err == nil {
			t.Fatal("expected error when route definition patterns are missing")
		}
		if !strings.Contains(err.Error(), "Vorma.ClientRouteDefinitionPatterns is required") {
			t.Fatalf("error = %q, expected required-patterns message", err)
		}
	})

	t.Run("returns error when route definition patterns contain only whitespace", func(t *testing.T) {
		v := &vormaruntime.Vorma{
			Config: &vormaruntime.VormaConfig{
				ClientRouteDefinitionPatterns: []string{" ", "\n\t"},
			},
			Log: testLogger(),
		}

		_, err := resolveClientRouteDefinitionFiles(v)
		if err == nil {
			t.Fatal("expected error when route definition patterns contain only whitespace")
		}
		if !strings.Contains(err.Error(), "Vorma.ClientRouteDefinitionPatterns cannot contain only empty values") {
			t.Fatalf("error = %q, expected whitespace-only-patterns message", err)
		}
	})

	t.Run("trims and deduplicates patterns and returns sorted cleaned file list", func(t *testing.T) {
		expandedPatterns := make([]string, 0, 1)
		routeDefinitionsFileResolutionDeps.expandRouteDefinitionPattern = func(pattern string) ([]string, error) {
			expandedPatterns = append(expandedPatterns, pattern)
			return []string{
				filepath.Join("frontend", "src", "routes", "..", "routes", "b.vorma.routes.ts"),
				filepath.Join("frontend", "src", "routes", "nested"),
				filepath.Join("frontend", "src", "routes", "a.vorma.routes.ts"),
			}, nil
		}
		routeDefinitionsFileResolutionDeps.statRouteDefinitionPath = func(path string) (fs.FileInfo, error) {
			return staticFileInfo{
				name:  filepath.Base(path),
				mode:  0o644,
				isDir: strings.HasSuffix(path, "nested"),
			}, nil
		}

		v := &vormaruntime.Vorma{
			Config: &vormaruntime.VormaConfig{
				ClientRouteDefinitionPatterns: []string{
					" frontend/src/**/*vorma.routes.ts ",
					"frontend/src/**/*vorma.routes.ts",
					" frontend/src/vorma.routes.ts ",
				},
			},
			Log: testLogger(),
		}

		files, err := resolveClientRouteDefinitionFiles(v)
		if err != nil {
			t.Fatalf("resolveClientRouteDefinitionFiles returned error: %v", err)
		}

		if !slices.Equal(expandedPatterns, []string{"frontend/src/**/*vorma.routes.ts"}) {
			t.Fatalf("expandedPatterns = %#v, want one trimmed glob pattern", expandedPatterns)
		}

		wantFiles := []string{
			"frontend/src/routes/a.vorma.routes.ts",
			"frontend/src/routes/b.vorma.routes.ts",
			"frontend/src/vorma.routes.ts",
		}
		if !slices.Equal(files, wantFiles) {
			t.Fatalf("files = %#v, want %#v", files, wantFiles)
		}
	})

	t.Run("returns expansion error for glob pattern", func(t *testing.T) {
		expectedErr := errors.New("glob expansion failed")
		routeDefinitionsFileResolutionDeps.expandRouteDefinitionPattern = func(string) ([]string, error) {
			return nil, expectedErr
		}
		routeDefinitionsFileResolutionDeps.statRouteDefinitionPath = func(string) (fs.FileInfo, error) {
			t.Fatal("did not expect stat when glob expansion fails")
			return nil, nil
		}

		v := &vormaruntime.Vorma{
			Config: &vormaruntime.VormaConfig{
				ClientRouteDefinitionPatterns: []string{"frontend/src/**/*vorma.routes.ts"},
			},
			Log: testLogger(),
		}

		_, err := resolveClientRouteDefinitionFiles(v)
		if err == nil {
			t.Fatal("expected expansion error")
		}
		if !strings.Contains(err.Error(), "expand route definition pattern \"frontend/src/**/*vorma.routes.ts\"") {
			t.Fatalf("error = %q, expected expansion context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped expansion error", err)
		}
	})

	t.Run("returns stat error for matched file from glob pattern", func(t *testing.T) {
		expectedErr := errors.New("stat failed")
		routeDefinitionsFileResolutionDeps.expandRouteDefinitionPattern = func(string) ([]string, error) {
			return []string{"frontend/src/routes/matched.vorma.routes.ts"}, nil
		}
		routeDefinitionsFileResolutionDeps.statRouteDefinitionPath = func(string) (fs.FileInfo, error) {
			return nil, expectedErr
		}

		v := &vormaruntime.Vorma{
			Config: &vormaruntime.VormaConfig{
				ClientRouteDefinitionPatterns: []string{"frontend/src/**/*vorma.routes.ts"},
			},
			Log: testLogger(),
		}

		_, err := resolveClientRouteDefinitionFiles(v)
		if err == nil {
			t.Fatal("expected stat error")
		}
		if !strings.Contains(err.Error(), "stat route definition path \"frontend/src/routes/matched.vorma.routes.ts\"") {
			t.Fatalf("error = %q, expected stat context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped stat error", err)
		}
	})

	t.Run("returns stat error for explicit route definition file", func(t *testing.T) {
		expectedErr := errors.New("stat failed")
		routeDefinitionsFileResolutionDeps.expandRouteDefinitionPattern = func(string) ([]string, error) {
			t.Fatal("did not expect glob expansion for explicit route definition file")
			return nil, nil
		}
		routeDefinitionsFileResolutionDeps.statRouteDefinitionPath = func(path string) (fs.FileInfo, error) {
			if path != "frontend/src/vorma.routes.ts" {
				t.Fatalf("path = %q, want %q", path, "frontend/src/vorma.routes.ts")
			}
			return nil, expectedErr
		}

		v := &vormaruntime.Vorma{
			Config: &vormaruntime.VormaConfig{
				ClientRouteDefinitionPatterns: []string{" frontend/src/vorma.routes.ts "},
			},
			Log: testLogger(),
		}

		_, err := resolveClientRouteDefinitionFiles(v)
		if err == nil {
			t.Fatal("expected stat error for explicit route definition file")
		}
		if !strings.Contains(err.Error(), "stat route definition path \"frontend/src/vorma.routes.ts\"") {
			t.Fatalf("error = %q, expected stat context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped stat error", err)
		}
	})

	t.Run("returns error for explicit route definition directory", func(t *testing.T) {
		routeDefinitionsFileResolutionDeps.expandRouteDefinitionPattern = func(string) ([]string, error) {
			t.Fatal("did not expect glob expansion for explicit route definition directory")
			return nil, nil
		}
		routeDefinitionsFileResolutionDeps.statRouteDefinitionPath = func(path string) (fs.FileInfo, error) {
			if path != "frontend/src/routes" {
				t.Fatalf("path = %q, want %q", path, "frontend/src/routes")
			}
			return staticFileInfo{
				name:  filepath.Base(path),
				mode:  fs.ModeDir,
				isDir: true,
			}, nil
		}

		v := &vormaruntime.Vorma{
			Config: &vormaruntime.VormaConfig{
				ClientRouteDefinitionPatterns: []string{"frontend/src/routes"},
			},
			Log: testLogger(),
		}

		_, err := resolveClientRouteDefinitionFiles(v)
		if err == nil {
			t.Fatal("expected explicit-directory error")
		}
		if !strings.Contains(err.Error(), "route definition path \"frontend/src/routes\" is a directory") {
			t.Fatalf("error = %q, expected explicit-directory message", err)
		}
	})

	t.Run("returns no-match error when glob matches only directories", func(t *testing.T) {
		routeDefinitionsFileResolutionDeps.expandRouteDefinitionPattern = func(string) ([]string, error) {
			return []string{"frontend/src/routes/nested"}, nil
		}
		routeDefinitionsFileResolutionDeps.statRouteDefinitionPath = func(path string) (fs.FileInfo, error) {
			return staticFileInfo{
				name:  filepath.Base(path),
				mode:  fs.ModeDir,
				isDir: true,
			}, nil
		}

		v := &vormaruntime.Vorma{
			Config: &vormaruntime.VormaConfig{
				ClientRouteDefinitionPatterns: []string{" frontend/src/**/*vorma.routes.ts "},
			},
			Log: testLogger(),
		}

		_, err := resolveClientRouteDefinitionFiles(v)
		if err == nil {
			t.Fatal("expected no-match error")
		}
		if !strings.Contains(err.Error(), "no route definition files matched patterns: frontend/src/**/*vorma.routes.ts") {
			t.Fatalf("error = %q, expected no-match message with trimmed pattern", err)
		}
	})
}

func TestParseClientRoutes_OrchestratesPipelineSteps(t *testing.T) {
	originalRouteParsingPipelineDeps := routeParsingPipelineDeps
	t.Cleanup(func() {
		routeParsingPipelineDeps = originalRouteParsingPipelineDeps
	})

	v := &vormaruntime.Vorma{
		Config: &vormaruntime.VormaConfig{
			ClientRouteDefinitionPatterns: []string{
				"frontend/src/**/*vorma.routes.ts",
			},
		},
		Log: testLogger(),
	}

	t.Run("runs resolve-parse-warn-merge flow", func(t *testing.T) {
		observedSteps := make([]string, 0, 4)
		routeCalls := []routeCall{
			{
				Pattern: "/home",
				Module:  "./routes/home.tsx",
				Key:     "default",
			},
		}
		unresolvedRoutes := []unresolvedRouteCall{
			{
				Pattern:       "/dynamic",
				RawModuleExpr: "getPath(...)",
				Reason:        "cannot statically analyze",
			},
		}
		expectedPaths := map[string]*vormaruntime.Path{
			"/home": {
				OriginalPattern: "/home",
				SrcPath:         "frontend/src/routes/home.tsx",
				ExportKey:       "default",
			},
		}

		routeParsingPipelineDeps.resolveClientRouteDefinitionFiles = func(_ *vormaruntime.Vorma) ([]string, error) {
			observedSteps = append(observedSteps, "resolve")
			return []string{"frontend/src/vorma.routes.ts"}, nil
		}
		routeParsingPipelineDeps.parseRouteDefinitionFileIntoCalls = func(
			_ *vormaruntime.Vorma,
			routeDefinitionFile string,
		) (parsedRouteDefinitionsCode, error) {
			observedSteps = append(observedSteps, "parse")
			if routeDefinitionFile != "frontend/src/vorma.routes.ts" {
				t.Fatalf("routeDefinitionFile = %q, want %q", routeDefinitionFile, "frontend/src/vorma.routes.ts")
			}
			return parsedRouteDefinitionsCode{
				routeCalls:       routeCalls,
				unresolvedRoutes: unresolvedRoutes,
			}, nil
		}
		routeParsingPipelineDeps.warnUnresolvedRouteCalls = func(
			_ *vormaruntime.Vorma,
			routeDefinitionFile string,
			unresolved []unresolvedRouteCall,
		) {
			observedSteps = append(observedSteps, "warn")
			if routeDefinitionFile != "frontend/src/vorma.routes.ts" {
				t.Fatalf("routeDefinitionFile = %q, want %q", routeDefinitionFile, "frontend/src/vorma.routes.ts")
			}
			if len(unresolved) != 1 || unresolved[0].Pattern != "/dynamic" {
				t.Fatalf("warn unresolved = %#v, want dynamic unresolved route", unresolved)
			}
		}
		routeParsingPipelineDeps.mergeRouteCallsIntoPaths = func(
			_ *vormaruntime.Vorma,
			paths map[string]*vormaruntime.Path,
			routeDefinitionFile string,
			calls []routeCall,
		) error {
			observedSteps = append(observedSteps, "merge")
			if routeDefinitionFile != "frontend/src/vorma.routes.ts" {
				t.Fatalf("routeDefinitionFile = %q, want %q", routeDefinitionFile, "frontend/src/vorma.routes.ts")
			}
			if len(calls) != 1 || calls[0].Pattern != "/home" {
				t.Fatalf("merge route calls = %#v, want /home route call", calls)
			}
			for pattern, pathValue := range expectedPaths {
				paths[pattern] = pathValue
			}
			return nil
		}
		paths, err := parseClientRoutes(v)
		if err != nil {
			t.Fatalf("parseClientRoutes returned error: %v", err)
		}
		if gotPath := paths["/home"]; gotPath == nil || gotPath.SrcPath != "frontend/src/routes/home.tsx" {
			t.Fatalf("paths[/home] = %#v, want frontend/src/routes/home.tsx", gotPath)
		}
		if !slices.Equal(observedSteps, []string{"resolve", "parse", "warn", "merge"}) {
			t.Fatalf("observed steps = %#v, want resolve->parse->warn->merge", observedSteps)
		}
	})

	t.Run("returns resolver error and stops flow", func(t *testing.T) {
		expectedErr := errors.New("resolve failed")
		routeParsingPipelineDeps.resolveClientRouteDefinitionFiles = func(*vormaruntime.Vorma) ([]string, error) {
			return nil, expectedErr
		}
		routeParsingPipelineDeps.parseRouteDefinitionFileIntoCalls = func(*vormaruntime.Vorma, string) (parsedRouteDefinitionsCode, error) {
			t.Fatal("did not expect parse step after resolver error")
			return parsedRouteDefinitionsCode{}, nil
		}
		routeParsingPipelineDeps.warnUnresolvedRouteCalls = func(*vormaruntime.Vorma, string, []unresolvedRouteCall) {
			t.Fatal("did not expect warning step after resolver error")
		}
		routeParsingPipelineDeps.mergeRouteCallsIntoPaths = func(*vormaruntime.Vorma, map[string]*vormaruntime.Path, string, []routeCall) error {
			t.Fatal("did not expect merge step after resolver error")
			return nil
		}

		_, err := parseClientRoutes(v)
		if err == nil {
			t.Fatal("expected parseClientRoutes to return resolver error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped resolver error", err)
		}
	})

	t.Run("returns parse-file error without warning/merge steps", func(t *testing.T) {
		expectedErr := errors.New("parse failed")
		routeParsingPipelineDeps.resolveClientRouteDefinitionFiles = func(*vormaruntime.Vorma) ([]string, error) {
			return []string{"frontend/src/vorma.routes.ts"}, nil
		}
		routeParsingPipelineDeps.parseRouteDefinitionFileIntoCalls = func(*vormaruntime.Vorma, string) (parsedRouteDefinitionsCode, error) {
			return parsedRouteDefinitionsCode{}, expectedErr
		}
		routeParsingPipelineDeps.warnUnresolvedRouteCalls = func(*vormaruntime.Vorma, string, []unresolvedRouteCall) {
			t.Fatal("did not expect warning step after parse error")
		}
		routeParsingPipelineDeps.mergeRouteCallsIntoPaths = func(*vormaruntime.Vorma, map[string]*vormaruntime.Path, string, []routeCall) error {
			t.Fatal("did not expect merge step after parse error")
			return nil
		}

		_, err := parseClientRoutes(v)
		if err == nil {
			t.Fatal("expected parseClientRoutes to return parse error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected parse error", err)
		}
	})

	t.Run("returns merge-paths error after warning step", func(t *testing.T) {
		expectedErr := errors.New("merge paths failed")
		warnCalled := false
		routeParsingPipelineDeps.resolveClientRouteDefinitionFiles = func(*vormaruntime.Vorma) ([]string, error) {
			return []string{"frontend/src/vorma.routes.ts"}, nil
		}
		routeParsingPipelineDeps.parseRouteDefinitionFileIntoCalls = func(*vormaruntime.Vorma, string) (parsedRouteDefinitionsCode, error) {
			return parsedRouteDefinitionsCode{
				routeCalls: []routeCall{
					{
						Pattern: "/home",
						Module:  "./routes/home.tsx",
						Key:     "default",
					},
				},
				unresolvedRoutes: []unresolvedRouteCall{
					{
						Pattern:       "/dynamic",
						RawModuleExpr: "getPath(...)",
						Reason:        "cannot statically analyze",
					},
				},
			}, nil
		}
		routeParsingPipelineDeps.warnUnresolvedRouteCalls = func(*vormaruntime.Vorma, string, []unresolvedRouteCall) {
			warnCalled = true
		}
		routeParsingPipelineDeps.mergeRouteCallsIntoPaths = func(*vormaruntime.Vorma, map[string]*vormaruntime.Path, string, []routeCall) error {
			return expectedErr
		}

		_, err := parseClientRoutes(v)
		if err == nil {
			t.Fatal("expected parseClientRoutes to return merge-paths error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected merge-paths error", err)
		}
		if !warnCalled {
			t.Fatal("expected warning step before merge-paths error")
		}
	})
}

func TestParseRouteDefinitionsCodeIntoCalls(t *testing.T) {
	originalRouteDefinitionsCodeParsingDeps := routeDefinitionsCodeParsingDeps
	t.Cleanup(func() {
		routeDefinitionsCodeParsingDeps = originalRouteDefinitionsCodeParsingDeps
	})

	v := &vormaruntime.Vorma{Log: testLogger()}

	t.Run("transforms and extracts calls", func(t *testing.T) {
		routeDefinitionsCodeParsingDeps.transformRouteDefinitionsCode = func(_ *vormaruntime.Vorma, code []byte) (string, error) {
			if string(code) != "raw route defs" {
				t.Fatalf("transform code = %q, want %q", string(code), "raw route defs")
			}
			return "transformed route defs", nil
		}
		routeDefinitionsCodeParsingDeps.extractRouteCallsFromTransformedCode = func(code string) ([]routeCall, []unresolvedRouteCall, error) {
			if code != "transformed route defs" {
				t.Fatalf("extract code = %q, want %q", code, "transformed route defs")
			}
			return []routeCall{
					{
						Pattern: "/home",
						Module:  "./routes/home.tsx",
						Key:     "default",
					},
				}, []unresolvedRouteCall{
					{
						Pattern:       "/dynamic",
						RawModuleExpr: "getPath(...)",
						Reason:        "cannot statically analyze",
					},
				}, nil
		}

		parsed, err := parseRouteDefinitionsCodeIntoCalls(v, []byte("raw route defs"))
		if err != nil {
			t.Fatalf("parseRouteDefinitionsCodeIntoCalls returned error: %v", err)
		}
		if len(parsed.routeCalls) != 1 || parsed.routeCalls[0].Pattern != "/home" {
			t.Fatalf("route calls = %#v, want /home route call", parsed.routeCalls)
		}
		if len(parsed.unresolvedRoutes) != 1 || parsed.unresolvedRoutes[0].Pattern != "/dynamic" {
			t.Fatalf("unresolved routes = %#v, want /dynamic unresolved route", parsed.unresolvedRoutes)
		}
	})

	t.Run("returns transform error and skips extract", func(t *testing.T) {
		expectedErr := errors.New("transform failed")
		routeDefinitionsCodeParsingDeps.transformRouteDefinitionsCode = func(*vormaruntime.Vorma, []byte) (string, error) {
			return "", expectedErr
		}
		routeDefinitionsCodeParsingDeps.extractRouteCallsFromTransformedCode = func(string) ([]routeCall, []unresolvedRouteCall, error) {
			t.Fatal("did not expect extract step after transform error")
			return nil, nil, nil
		}

		_, err := parseRouteDefinitionsCodeIntoCalls(v, []byte("raw route defs"))
		if err == nil {
			t.Fatal("expected parseRouteDefinitionsCodeIntoCalls to return transform error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected transform error", err)
		}
	})

	t.Run("wraps extract error", func(t *testing.T) {
		expectedErr := errors.New("extract failed")
		routeDefinitionsCodeParsingDeps.transformRouteDefinitionsCode = func(*vormaruntime.Vorma, []byte) (string, error) {
			return "transformed route defs", nil
		}
		routeDefinitionsCodeParsingDeps.extractRouteCallsFromTransformedCode = func(string) ([]routeCall, []unresolvedRouteCall, error) {
			return nil, nil, expectedErr
		}

		_, err := parseRouteDefinitionsCodeIntoCalls(v, []byte("raw route defs"))
		if err == nil {
			t.Fatal("expected parseRouteDefinitionsCodeIntoCalls to return extract error")
		}
		if !strings.Contains(err.Error(), "extract route calls") {
			t.Fatalf("error = %q, expected extract context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped extract error", err)
		}
	})
}

func TestParseRouteDefinitionFileIntoCalls_ReturnsReadError(t *testing.T) {
	v := &vormaruntime.Vorma{
		Log: testLogger(),
	}

	_, err := parseRouteDefinitionFileIntoCalls(v, filepath.Join(t.TempDir(), "missing.vorma.routes.ts"))
	if err == nil {
		t.Fatal("expected read error for missing route definition file")
	}
	if !strings.Contains(err.Error(), "read route definitions file") {
		t.Fatalf("error = %q, expected read-error context", err)
	}
}

func TestMergeRouteCallsIntoPaths(t *testing.T) {
	t.Run("returns error when a route call has no module argument", func(t *testing.T) {
		v := &vormaruntime.Vorma{
			Config: &vormaruntime.VormaConfig{
				ClientRouteDefinitionPatterns: []string{"frontend/src/**/*vorma.routes.ts"},
			},
			Log: testLogger(),
		}

		paths := map[string]*vormaruntime.Path{}
		err := mergeRouteCallsIntoPaths(v, paths, "frontend/src/vorma.routes.ts", []routeCall{{
			Pattern: "/missing-module",
			Module:  "",
			Key:     "default",
		}})
		if err == nil {
			t.Fatal("expected mergeRouteCallsIntoPaths to fail when module argument is missing")
		}
		if !strings.Contains(err.Error(), "component module is required for pattern: /missing-module") {
			t.Fatalf("error = %q, expected missing-module context", err)
		}
	})

	t.Run("returns error when route pattern is duplicated", func(t *testing.T) {
		originalRouteModuleResolutionDeps := routeModuleResolutionDeps
		t.Cleanup(func() {
			routeModuleResolutionDeps = originalRouteModuleResolutionDeps
		})

		routeModuleResolutionDeps.computeRelativeModulePath = func(string, string) (string, error) {
			return "frontend/src/routes/example.tsx", nil
		}
		routeModuleResolutionDeps.statRouteModulePath = func(string) (fs.FileInfo, error) {
			return nil, nil
		}

		v := &vormaruntime.Vorma{
			Config: &vormaruntime.VormaConfig{
				ClientRouteDefinitionPatterns: []string{"frontend/src/**/*vorma.routes.ts"},
			},
			Log: testLogger(),
		}

		paths := map[string]*vormaruntime.Path{}
		err := mergeRouteCallsIntoPaths(v, paths, "frontend/src/vorma.routes.ts", []routeCall{
			{
				Pattern: "/dup",
				Module:  "./routes/first.tsx",
				Key:     "default",
			},
			{
				Pattern: "/dup",
				Module:  "./routes/second.tsx",
				Key:     "default",
			},
		})
		if err == nil {
			t.Fatal("expected mergeRouteCallsIntoPaths to fail on duplicate route pattern")
		}
		if !strings.Contains(err.Error(), "duplicate route pattern: /dup") {
			t.Fatalf("error = %q, expected duplicate-pattern context", err)
		}
	})
}
