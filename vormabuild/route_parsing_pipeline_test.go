package vormabuild

import (
	"errors"
	"io/fs"
	"slices"
	"strings"
	"testing"

	"github.com/vormadev/vorma/vormaruntime"
)

func TestParseClientRoutes_OrchestratesPipelineSteps(t *testing.T) {
	originalRouteParsingPipelineDeps := routeParsingPipelineDeps
	t.Cleanup(func() {
		routeParsingPipelineDeps = originalRouteParsingPipelineDeps
	})

	v := &vormaruntime.Vorma{
		Config: &vormaruntime.VormaConfig{
			ClientRouteDefsFile: "frontend/src/vorma.routes.ts",
		},
		Log: testLogger(),
	}

	t.Run("runs read-parse-warn-build flow", func(t *testing.T) {
		observedSteps := make([]string, 0, 4)
		routeCalls := []RouteCall{
			{
				Pattern: "/home",
				Module:  "./routes/home.tsx",
				Key:     "default",
			},
		}
		unresolvedRoutes := []UnresolvedRouteCall{
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

		routeParsingPipelineDeps.readClientRouteDefinitionsFile = func(path string) ([]byte, error) {
			observedSteps = append(observedSteps, "read")
			if path != "frontend/src/vorma.routes.ts" {
				t.Fatalf("read path = %q, want %q", path, "frontend/src/vorma.routes.ts")
			}
			return []byte("route defs"), nil
		}
		routeParsingPipelineDeps.parseRouteDefinitionsCodeIntoCalls = func(
			_ *vormaruntime.Vorma,
			code []byte,
		) (parsedRouteDefinitionsCode, error) {
			observedSteps = append(observedSteps, "parse")
			if string(code) != "route defs" {
				t.Fatalf("parse code = %q, want %q", string(code), "route defs")
			}
			return parsedRouteDefinitionsCode{
				routeCalls:       routeCalls,
				unresolvedRoutes: unresolvedRoutes,
			}, nil
		}
		routeParsingPipelineDeps.warnUnresolvedRouteCalls = func(_ *vormaruntime.Vorma, unresolved []UnresolvedRouteCall) {
			observedSteps = append(observedSteps, "warn")
			if len(unresolved) != 1 || unresolved[0].Pattern != "/dynamic" {
				t.Fatalf("warn unresolved = %#v, want dynamic unresolved route", unresolved)
			}
		}
		routeParsingPipelineDeps.buildPathsFromRouteCalls = func(
			_ *vormaruntime.Vorma,
			calls []RouteCall,
		) (map[string]*vormaruntime.Path, error) {
			observedSteps = append(observedSteps, "build")
			if len(calls) != 1 || calls[0].Pattern != "/home" {
				t.Fatalf("build route calls = %#v, want /home route call", calls)
			}
			return expectedPaths, nil
		}

		paths, err := parseClientRoutes(v)
		if err != nil {
			t.Fatalf("parseClientRoutes returned error: %v", err)
		}
		if gotPath := paths["/home"]; gotPath == nil || gotPath.SrcPath != "frontend/src/routes/home.tsx" {
			t.Fatalf("paths[/home] = %#v, want frontend/src/routes/home.tsx", gotPath)
		}
		if !slices.Equal(observedSteps, []string{"read", "parse", "warn", "build"}) {
			t.Fatalf("observed steps = %#v, want read->parse->warn->build", observedSteps)
		}
	})

	t.Run("wraps read-file error and stops flow", func(t *testing.T) {
		expectedErr := errors.New("read failed")
		routeParsingPipelineDeps.readClientRouteDefinitionsFile = func(string) ([]byte, error) {
			return nil, expectedErr
		}
		routeParsingPipelineDeps.parseRouteDefinitionsCodeIntoCalls = func(*vormaruntime.Vorma, []byte) (parsedRouteDefinitionsCode, error) {
			t.Fatal("did not expect parse step after read-file error")
			return parsedRouteDefinitionsCode{}, nil
		}
		routeParsingPipelineDeps.warnUnresolvedRouteCalls = func(*vormaruntime.Vorma, []UnresolvedRouteCall) {
			t.Fatal("did not expect warning step after read-file error")
		}
		routeParsingPipelineDeps.buildPathsFromRouteCalls = func(*vormaruntime.Vorma, []RouteCall) (map[string]*vormaruntime.Path, error) {
			t.Fatal("did not expect build step after read-file error")
			return nil, nil
		}

		_, err := parseClientRoutes(v)
		if err == nil {
			t.Fatal("expected parseClientRoutes to return read-file error")
		}
		if !strings.Contains(err.Error(), "read file") {
			t.Fatalf("error = %q, expected read-file context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped read-file error", err)
		}
	})

	t.Run("returns parse error without warning/build steps", func(t *testing.T) {
		expectedErr := errors.New("parse failed")
		routeParsingPipelineDeps.readClientRouteDefinitionsFile = func(string) ([]byte, error) {
			return []byte("route defs"), nil
		}
		routeParsingPipelineDeps.parseRouteDefinitionsCodeIntoCalls = func(*vormaruntime.Vorma, []byte) (parsedRouteDefinitionsCode, error) {
			return parsedRouteDefinitionsCode{}, expectedErr
		}
		routeParsingPipelineDeps.warnUnresolvedRouteCalls = func(*vormaruntime.Vorma, []UnresolvedRouteCall) {
			t.Fatal("did not expect warning step after parse error")
		}
		routeParsingPipelineDeps.buildPathsFromRouteCalls = func(*vormaruntime.Vorma, []RouteCall) (map[string]*vormaruntime.Path, error) {
			t.Fatal("did not expect build step after parse error")
			return nil, nil
		}

		_, err := parseClientRoutes(v)
		if err == nil {
			t.Fatal("expected parseClientRoutes to return parse error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected parse error", err)
		}
	})

	t.Run("returns build-paths error after warning step", func(t *testing.T) {
		expectedErr := errors.New("build paths failed")
		warnCalled := false
		routeParsingPipelineDeps.readClientRouteDefinitionsFile = func(string) ([]byte, error) {
			return []byte("route defs"), nil
		}
		routeParsingPipelineDeps.parseRouteDefinitionsCodeIntoCalls = func(*vormaruntime.Vorma, []byte) (parsedRouteDefinitionsCode, error) {
			return parsedRouteDefinitionsCode{
				routeCalls: []RouteCall{
					{
						Pattern: "/home",
						Module:  "./routes/home.tsx",
						Key:     "default",
					},
				},
				unresolvedRoutes: []UnresolvedRouteCall{
					{
						Pattern:       "/dynamic",
						RawModuleExpr: "getPath(...)",
						Reason:        "cannot statically analyze",
					},
				},
			}, nil
		}
		routeParsingPipelineDeps.warnUnresolvedRouteCalls = func(*vormaruntime.Vorma, []UnresolvedRouteCall) {
			warnCalled = true
		}
		routeParsingPipelineDeps.buildPathsFromRouteCalls = func(*vormaruntime.Vorma, []RouteCall) (map[string]*vormaruntime.Path, error) {
			return nil, expectedErr
		}

		_, err := parseClientRoutes(v)
		if err == nil {
			t.Fatal("expected parseClientRoutes to return build-paths error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected build-paths error", err)
		}
		if !warnCalled {
			t.Fatal("expected warning step before build-paths error")
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
		routeDefinitionsCodeParsingDeps.extractRouteCallsFromTransformedCode = func(code string) ([]RouteCall, []UnresolvedRouteCall, error) {
			if code != "transformed route defs" {
				t.Fatalf("extract code = %q, want %q", code, "transformed route defs")
			}
			return []RouteCall{
					{
						Pattern: "/home",
						Module:  "./routes/home.tsx",
						Key:     "default",
					},
				}, []UnresolvedRouteCall{
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
		routeDefinitionsCodeParsingDeps.extractRouteCallsFromTransformedCode = func(string) ([]RouteCall, []UnresolvedRouteCall, error) {
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
		routeDefinitionsCodeParsingDeps.extractRouteCallsFromTransformedCode = func(string) ([]RouteCall, []UnresolvedRouteCall, error) {
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

func TestBuildPathsFromRouteCalls(t *testing.T) {
	t.Run("returns error when a route call has no module argument", func(t *testing.T) {
		v := &vormaruntime.Vorma{
			Config: &vormaruntime.VormaConfig{
				ClientRouteDefsFile: "frontend/src/vorma.routes.ts",
			},
			Log: testLogger(),
		}

		_, err := buildPathsFromRouteCalls(v, []RouteCall{{
			Pattern: "/missing-module",
			Module:  "",
			Key:     "default",
		}})
		if err == nil {
			t.Fatal("expected buildPathsFromRouteCalls to fail when module argument is missing")
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
				ClientRouteDefsFile: "frontend/src/vorma.routes.ts",
			},
			Log: testLogger(),
		}

		_, err := buildPathsFromRouteCalls(v, []RouteCall{
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
			t.Fatal("expected buildPathsFromRouteCalls to fail on duplicate route pattern")
		}
		if !strings.Contains(err.Error(), "duplicate route pattern: /dup") {
			t.Fatalf("error = %q, expected duplicate-pattern context", err)
		}
	})
}
