package routeparse

import (
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
)

func routeParseTestPathFromWorkingDirectory(
	tb testing.TB,
	repositoryRelativePath string,
) string {
	tb.Helper()
	return filepath.ToSlash(
		filepath.Clean(filepath.FromSlash(repositoryRelativePath)),
	)
}

func newRouteParsingExecutorForTest(
	mutateDependencies func(*routeParsingExecutorDependencies),
) routeParsingExecutor {
	dependencies := routeParsingExecutorDependencies{}
	if mutateDependencies != nil {
		mutateDependencies(&dependencies)
	}
	return newRouteParsingExecutor(dependencies)
}

func TestResolveClientRouteDefinitionFiles(t *testing.T) {
	t.Run("returns required error when runtime is nil", func(t *testing.T) {
		_, err := resolveClientRouteDefinitionFiles(nil)
		if err == nil {
			t.Fatal("expected error when runtime is nil")
		}
		if !strings.Contains(err.Error(), "vorma runtime is required") {
			t.Fatalf("error = %q, expected nil-runtime message", err)
		}
	})

	t.Run(
		"returns required error when runtime config is nil",
		func(t *testing.T) {
			v := &vormaruntime.Vorma{
				Log: testLogger(),
			}

			_, err := resolveClientRouteDefinitionFiles(v)
			if err == nil {
				t.Fatal("expected error when runtime config is nil")
			}
			if !strings.Contains(err.Error(), "vorma config is required") {
				t.Fatalf("error = %q, expected nil-config message", err)
			}
		},
	)

	t.Run(
		"returns parse error when route definition patterns are missing",
		func(t *testing.T) {
			rawConfig := defaultRawVormaConfigJSONForRouteParseTests()
			rawConfig.ClientRouteDefinitionPatterns = []string{}
			_, parseError := parseVormaConfigForRouteParseTests(t, rawConfig)
			if parseError == nil {
				t.Fatal(
					"expected parse error when route definition patterns are missing",
				)
			}
			if !strings.Contains(parseError.Error(), "ClientRouteDefinitionPatterns") {
				t.Fatalf("error = %q, expected required-patterns parse error", parseError)
			}
		},
	)

	t.Run(
		"returns parse error when route definition patterns contain only whitespace",
		func(t *testing.T) {
			rawConfig := defaultRawVormaConfigJSONForRouteParseTests()
			rawConfig.ClientRouteDefinitionPatterns = []string{" ", "\n\t"}
			_, parseError := parseVormaConfigForRouteParseTests(t, rawConfig)
			if parseError == nil {
				t.Fatal(
					"expected parse error for route definition patterns containing only whitespace",
				)
			}
			if !strings.Contains(parseError.Error(), "cannot be empty or whitespace") {
				t.Fatalf(
					"error = %q, expected whitespace-only-patterns parse error",
					parseError,
				)
			}
		},
	)

	t.Run(
		"returns sorted cleaned file list for already-valid patterns",
		func(t *testing.T) {
			expandedPatterns := make([]string, 0, 2)
			executor := newRouteParsingExecutorForTest(
				func(dependencies *routeParsingExecutorDependencies) {
					dependencies.expandRouteDefinitionPattern = func(
						pattern string,
					) ([]string, error) {
						expandedPatterns = append(expandedPatterns, pattern)
						return []string{
							routeParseTestPathFromWorkingDirectory(
								t,
								"frontend/src/routes/../routes/b.vorma.routes.ts",
							),
							routeParseTestPathFromWorkingDirectory(
								t,
								"frontend/src/routes/nested",
							),
							routeParseTestPathFromWorkingDirectory(
								t,
								"frontend/src/routes/a.vorma.routes.ts",
							),
						}, nil
					}
					dependencies.statRouteDefinitionPath = func(
						path string,
					) (fs.FileInfo, error) {
						return staticFileInfo{
							name:  filepath.Base(path),
							mode:  0o644,
							isDir: strings.HasSuffix(path, "nested"),
						}, nil
					}
				},
			)

			v := &vormaruntime.Vorma{
				Config: mustParsedVormaConfigForRouteParseTests(t, &vormaruntime.VormaConfigJSON{
					ClientRouteDefinitionPatterns: []string{
						"frontend/src/**/*vorma.routes.ts",
						"frontend/src/vorma.routes.ts",
					},
				}),
				Log: testLogger(),
			}

			files, err := executor.resolveClientRouteDefinitionFiles(v)
			if err != nil {
				t.Fatalf(
					"resolveClientRouteDefinitionFiles returned error: %v",
					err,
				)
			}

			if !slices.Equal(
				expandedPatterns,
				[]string{
					routeParseTestPathFromWorkingDirectory(
						t,
						"frontend/src/**/*vorma.routes.ts",
					),
				},
			) {
				t.Fatalf(
					"expandedPatterns = %#v, want one explicit glob pattern",
					expandedPatterns,
				)
			}

			wantFiles := []string{
				routeParseTestPathFromWorkingDirectory(
					t,
					"frontend/src/routes/a.vorma.routes.ts",
				),
				routeParseTestPathFromWorkingDirectory(
					t,
					"frontend/src/routes/b.vorma.routes.ts",
				),
				routeParseTestPathFromWorkingDirectory(
					t,
					"frontend/src/vorma.routes.ts",
				),
			}
			if !slices.Equal(files, wantFiles) {
				t.Fatalf("files = %#v, want %#v", files, wantFiles)
			}
		},
	)

	t.Run("returns expansion error for glob pattern", func(t *testing.T) {
		expectedErr := errors.New("glob expansion failed")
		executor := newRouteParsingExecutorForTest(
			func(dependencies *routeParsingExecutorDependencies) {
				dependencies.expandRouteDefinitionPattern = func(
					string,
				) ([]string, error) {
					return nil, expectedErr
				}
				dependencies.statRouteDefinitionPath = func(
					string,
				) (fs.FileInfo, error) {
					t.Fatal("did not expect stat when glob expansion fails")
					return nil, nil
				}
			},
		)

		v := &vormaruntime.Vorma{
			Config: mustParsedVormaConfigForRouteParseTests(t, &vormaruntime.VormaConfigJSON{
				ClientRouteDefinitionPatterns: []string{
					"frontend/src/**/*vorma.routes.ts",
				},
			}),
			Log: testLogger(),
		}

		_, err := executor.resolveClientRouteDefinitionFiles(v)
		if err == nil {
			t.Fatal("expected expansion error")
		}
		if !strings.Contains(
			err.Error(),
			"expand route definition pattern \""+
				routeParseTestPathFromWorkingDirectory(
					t,
					"frontend/src/**/*vorma.routes.ts",
				)+"\"",
		) {
			t.Fatalf("error = %q, expected expansion context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped expansion error", err)
		}
	})

	t.Run(
		"returns stat error for matched file from glob pattern",
		func(t *testing.T) {
			expectedErr := errors.New("stat failed")
			executor := newRouteParsingExecutorForTest(
				func(dependencies *routeParsingExecutorDependencies) {
					dependencies.expandRouteDefinitionPattern = func(
						string,
					) ([]string, error) {
						return []string{
							"frontend/src/routes/matched.vorma.routes.ts",
						}, nil
					}
					dependencies.statRouteDefinitionPath = func(
						string,
					) (fs.FileInfo, error) {
						return nil, expectedErr
					}
				},
			)

			v := &vormaruntime.Vorma{
				Config: mustParsedVormaConfigForRouteParseTests(t, &vormaruntime.VormaConfigJSON{
					ClientRouteDefinitionPatterns: []string{
						"frontend/src/**/*vorma.routes.ts",
					},
				}),
				Log: testLogger(),
			}

			_, err := executor.resolveClientRouteDefinitionFiles(v)
			if err == nil {
				t.Fatal("expected stat error")
			}
			if !strings.Contains(
				err.Error(),
				"stat route definition path \"frontend/src/routes/matched.vorma.routes.ts\"",
			) {
				t.Fatalf("error = %q, expected stat context", err)
			}
			if !errors.Is(err, expectedErr) {
				t.Fatalf("error = %v, expected wrapped stat error", err)
			}
		},
	)

	t.Run(
		"returns stat error for explicit route definition file",
		func(t *testing.T) {
			expectedErr := errors.New("stat failed")
			executor := newRouteParsingExecutorForTest(
				func(dependencies *routeParsingExecutorDependencies) {
					dependencies.expandRouteDefinitionPattern = func(
						string,
					) ([]string, error) {
						t.Fatal(
							"did not expect glob expansion for explicit route definition file",
						)
						return nil, nil
					}
					dependencies.statRouteDefinitionPath = func(
						path string,
					) (fs.FileInfo, error) {
						expectedPath := routeParseTestPathFromWorkingDirectory(
							t,
							"frontend/src/vorma.routes.ts",
						)
						if path != expectedPath {
							t.Fatalf(
								"path = %q, want %q",
								path,
								expectedPath,
							)
						}
						return nil, expectedErr
					}
				},
			)

			v := &vormaruntime.Vorma{
				Config: mustParsedVormaConfigForRouteParseTests(t, &vormaruntime.VormaConfigJSON{
					ClientRouteDefinitionPatterns: []string{
						"frontend/src/vorma.routes.ts",
					},
				}),
				Log: testLogger(),
			}

			_, err := executor.resolveClientRouteDefinitionFiles(v)
			if err == nil {
				t.Fatal(
					"expected stat error for explicit route definition file",
				)
			}
			if !strings.Contains(
				err.Error(),
				"stat route definition path \""+
					routeParseTestPathFromWorkingDirectory(
						t,
						"frontend/src/vorma.routes.ts",
					)+"\"",
			) {
				t.Fatalf("error = %q, expected stat context", err)
			}
			if !errors.Is(err, expectedErr) {
				t.Fatalf("error = %v, expected wrapped stat error", err)
			}
		},
	)

	t.Run(
		"returns error for explicit route definition directory",
		func(t *testing.T) {
			executor := newRouteParsingExecutorForTest(
				func(dependencies *routeParsingExecutorDependencies) {
					dependencies.expandRouteDefinitionPattern = func(
						string,
					) ([]string, error) {
						t.Fatal(
							"did not expect glob expansion for explicit route definition directory",
						)
						return nil, nil
					}
					dependencies.statRouteDefinitionPath = func(
						path string,
					) (fs.FileInfo, error) {
						expectedPath := routeParseTestPathFromWorkingDirectory(
							t,
							"frontend/src/routes",
						)
						if path != expectedPath {
							t.Fatalf(
								"path = %q, want %q",
								path,
								expectedPath,
							)
						}
						return staticFileInfo{
							name:  filepath.Base(path),
							mode:  fs.ModeDir,
							isDir: true,
						}, nil
					}
				},
			)

			v := &vormaruntime.Vorma{
				Config: mustParsedVormaConfigForRouteParseTests(t, &vormaruntime.VormaConfigJSON{
					ClientRouteDefinitionPatterns: []string{
						"frontend/src/routes",
					},
				}),
				Log: testLogger(),
			}

			_, err := executor.resolveClientRouteDefinitionFiles(v)
			if err == nil {
				t.Fatal("expected explicit-directory error")
			}
			if !strings.Contains(
				err.Error(),
				"route definition path \""+
					routeParseTestPathFromWorkingDirectory(
						t,
						"frontend/src/routes",
					)+"\" is a directory",
			) {
				t.Fatalf("error = %q, expected explicit-directory message", err)
			}
		},
	)

	t.Run(
		"returns no-match error when glob matches only directories",
		func(t *testing.T) {
			executor := newRouteParsingExecutorForTest(
				func(dependencies *routeParsingExecutorDependencies) {
					dependencies.expandRouteDefinitionPattern = func(
						string,
					) ([]string, error) {
						return []string{"frontend/src/routes/nested"}, nil
					}
					dependencies.statRouteDefinitionPath = func(
						path string,
					) (fs.FileInfo, error) {
						return staticFileInfo{
							name:  filepath.Base(path),
							mode:  fs.ModeDir,
							isDir: true,
						}, nil
					}
				},
			)

			v := &vormaruntime.Vorma{
				Config: mustParsedVormaConfigForRouteParseTests(t, &vormaruntime.VormaConfigJSON{
					ClientRouteDefinitionPatterns: []string{
						"frontend/src/**/*vorma.routes.ts",
					},
				}),
				Log: testLogger(),
			}

			_, err := executor.resolveClientRouteDefinitionFiles(v)
			if err == nil {
				t.Fatal("expected no-match error")
			}
			if !strings.Contains(
				err.Error(),
				"no route definition files matched patterns: "+
					routeParseTestPathFromWorkingDirectory(
						t,
						"frontend/src/**/*vorma.routes.ts",
					),
			) {
				t.Fatalf("error = %q, expected no-match message", err)
			}
		},
	)
}

func TestParseClientRoutes_OrchestratesPipelineSteps(t *testing.T) {
	v := &vormaruntime.Vorma{
		Config: mustParsedVormaConfigForRouteParseTests(t, &vormaruntime.VormaConfigJSON{
			ClientRouteDefinitionPatterns: []string{
				"frontend/src/**/*vorma.routes.ts",
			},
		}),
		Log: testLogger(),
	}

	t.Run("runs resolve-handle-merge flow", func(t *testing.T) {
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

		executor := newRouteParsingExecutorForTest(
			func(dependencies *routeParsingExecutorDependencies) {
				dependencies.resolveClientRouteDefinitionFiles = func(
					_ *vormaruntime.Vorma,
				) ([]string, error) {
					observedSteps = append(observedSteps, "resolve")
					return []string{"frontend/src/vorma.routes.ts"}, nil
				}
				dependencies.parseRouteDefinitionFileIntoCalls = func(
					_ *vormaruntime.Vorma,
					routeDefinitionFile string,
				) (parsedRouteDefinitionsCode, error) {
					observedSteps = append(observedSteps, "parse")
					if routeDefinitionFile != "frontend/src/vorma.routes.ts" {
						t.Fatalf(
							"routeDefinitionFile = %q, want %q",
							routeDefinitionFile,
							"frontend/src/vorma.routes.ts",
						)
					}
					return parsedRouteDefinitionsCode{
						routeCalls:       routeCalls,
						unresolvedRoutes: unresolvedRoutes,
					}, nil
				}
				dependencies.handleUnresolvedRouteCalls = func(
					_ *vormaruntime.Vorma,
					routeDefinitionFile string,
					unresolved []unresolvedRouteCall,
				) error {
					observedSteps = append(observedSteps, "handle-unresolved")
					if routeDefinitionFile != "frontend/src/vorma.routes.ts" {
						t.Fatalf(
							"routeDefinitionFile = %q, want %q",
							routeDefinitionFile,
							"frontend/src/vorma.routes.ts",
						)
					}
					if len(unresolved) != 1 ||
						unresolved[0].Pattern != "/dynamic" {
						t.Fatalf(
							"warn unresolved = %#v, want dynamic unresolved route",
							unresolved,
						)
					}
					return nil
				}
				dependencies.mergeRouteCallsIntoPaths = func(
					_ *vormaruntime.Vorma,
					paths map[string]*vormaruntime.Path,
					routeDefinitionFile string,
					calls []routeCall,
				) error {
					observedSteps = append(observedSteps, "merge")
					if routeDefinitionFile != "frontend/src/vorma.routes.ts" {
						t.Fatalf(
							"routeDefinitionFile = %q, want %q",
							routeDefinitionFile,
							"frontend/src/vorma.routes.ts",
						)
					}
					if len(calls) != 1 || calls[0].Pattern != "/home" {
						t.Fatalf(
							"merge route calls = %#v, want /home route call",
							calls,
						)
					}
					for pattern, pathValue := range expectedPaths {
						paths[pattern] = pathValue
					}
					return nil
				}
			},
		)

		paths, err := executor.ParseClientRoutes(v)
		if err != nil {
			t.Fatalf("parseClientRoutes returned error: %v", err)
		}
		if gotPath := paths["/home"]; gotPath == nil ||
			gotPath.SrcPath != "frontend/src/routes/home.tsx" {
			t.Fatalf(
				"paths[/home] = %#v, want frontend/src/routes/home.tsx",
				gotPath,
			)
		}
		if !slices.Equal(
			observedSteps,
			[]string{"resolve", "parse", "handle-unresolved", "merge"},
		) {
			t.Fatalf(
				"observed steps = %#v, want resolve->parse->handle-unresolved->merge",
				observedSteps,
			)
		}
	})

	t.Run("returns resolver error and stops flow", func(t *testing.T) {
		expectedErr := errors.New("resolve failed")
		executor := newRouteParsingExecutorForTest(
			func(dependencies *routeParsingExecutorDependencies) {
				dependencies.resolveClientRouteDefinitionFiles = func(
					*vormaruntime.Vorma,
				) ([]string, error) {
					return nil, expectedErr
				}
				dependencies.parseRouteDefinitionFileIntoCalls = func(
					*vormaruntime.Vorma,
					string,
				) (parsedRouteDefinitionsCode, error) {
					t.Fatal("did not expect parse step after resolver error")
					return parsedRouteDefinitionsCode{}, nil
				}
				dependencies.handleUnresolvedRouteCalls = func(
					*vormaruntime.Vorma,
					string,
					[]unresolvedRouteCall,
				) error {
					t.Fatal(
						"did not expect unresolved-route handling step after resolver error",
					)
					return nil
				}
				dependencies.mergeRouteCallsIntoPaths = func(
					*vormaruntime.Vorma,
					map[string]*vormaruntime.Path,
					string,
					[]routeCall,
				) error {
					t.Fatal("did not expect merge step after resolver error")
					return nil
				}
			},
		)

		_, err := executor.ParseClientRoutes(v)
		if err == nil {
			t.Fatal("expected parseClientRoutes to return resolver error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped resolver error", err)
		}
	})

	t.Run(
		"returns parse-file error without unresolved-route handling/merge steps",
		func(t *testing.T) {
			expectedErr := errors.New("parse failed")
			executor := newRouteParsingExecutorForTest(
				func(dependencies *routeParsingExecutorDependencies) {
					dependencies.resolveClientRouteDefinitionFiles = func(
						*vormaruntime.Vorma,
					) ([]string, error) {
						return []string{"frontend/src/vorma.routes.ts"}, nil
					}
					dependencies.parseRouteDefinitionFileIntoCalls = func(
						*vormaruntime.Vorma,
						string,
					) (parsedRouteDefinitionsCode, error) {
						return parsedRouteDefinitionsCode{}, expectedErr
					}
					dependencies.handleUnresolvedRouteCalls = func(
						*vormaruntime.Vorma,
						string,
						[]unresolvedRouteCall,
					) error {
						t.Fatal(
							"did not expect unresolved-route handling step after parse error",
						)
						return nil
					}
					dependencies.mergeRouteCallsIntoPaths = func(
						*vormaruntime.Vorma,
						map[string]*vormaruntime.Path,
						string,
						[]routeCall,
					) error {
						t.Fatal("did not expect merge step after parse error")
						return nil
					}
				},
			)

			_, err := executor.ParseClientRoutes(v)
			if err == nil {
				t.Fatal("expected parseClientRoutes to return parse error")
			}
			if !errors.Is(err, expectedErr) {
				t.Fatalf("error = %v, expected parse error", err)
			}
		},
	)

	t.Run(
		"returns merge-paths error after unresolved-route handling step",
		func(t *testing.T) {
			expectedErr := errors.New("merge paths failed")
			unresolvedHandlerCalled := false
			executor := newRouteParsingExecutorForTest(
				func(dependencies *routeParsingExecutorDependencies) {
					dependencies.resolveClientRouteDefinitionFiles = func(
						*vormaruntime.Vorma,
					) ([]string, error) {
						return []string{"frontend/src/vorma.routes.ts"}, nil
					}
					dependencies.parseRouteDefinitionFileIntoCalls = func(
						*vormaruntime.Vorma,
						string,
					) (parsedRouteDefinitionsCode, error) {
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
					dependencies.handleUnresolvedRouteCalls = func(
						*vormaruntime.Vorma,
						string,
						[]unresolvedRouteCall,
					) error {
						unresolvedHandlerCalled = true
						return nil
					}
					dependencies.mergeRouteCallsIntoPaths = func(
						*vormaruntime.Vorma,
						map[string]*vormaruntime.Path,
						string,
						[]routeCall,
					) error {
						return expectedErr
					}
				},
			)

			_, err := executor.ParseClientRoutes(v)
			if err == nil {
				t.Fatal(
					"expected parseClientRoutes to return merge-paths error",
				)
			}
			if !errors.Is(err, expectedErr) {
				t.Fatalf("error = %v, expected merge-paths error", err)
			}
			if !unresolvedHandlerCalled {
				t.Fatal(
					"expected unresolved-route handling step before merge-paths error",
				)
			}
		},
	)

	t.Run(
		"returns unresolved-route handling error and skips merge",
		func(t *testing.T) {
			expectedErr := errors.New("unresolved route policy failed")
			executor := newRouteParsingExecutorForTest(
				func(dependencies *routeParsingExecutorDependencies) {
					dependencies.resolveClientRouteDefinitionFiles = func(
						*vormaruntime.Vorma,
					) ([]string, error) {
						return []string{"frontend/src/vorma.routes.ts"}, nil
					}
					dependencies.parseRouteDefinitionFileIntoCalls = func(
						*vormaruntime.Vorma,
						string,
					) (parsedRouteDefinitionsCode, error) {
						return parsedRouteDefinitionsCode{
							unresolvedRoutes: []unresolvedRouteCall{
								{
									Pattern:       "/dynamic",
									RawModuleExpr: "getPath(...)",
									Reason:        "cannot statically analyze",
								},
							},
						}, nil
					}
					dependencies.handleUnresolvedRouteCalls = func(
						*vormaruntime.Vorma,
						string,
						[]unresolvedRouteCall,
					) error {
						return expectedErr
					}
					dependencies.mergeRouteCallsIntoPaths = func(
						*vormaruntime.Vorma,
						map[string]*vormaruntime.Path,
						string,
						[]routeCall,
					) error {
						t.Fatal(
							"did not expect merge step when unresolved-route handling fails",
						)
						return nil
					}
				},
			)

			_, err := executor.ParseClientRoutes(v)
			if err == nil {
				t.Fatal(
					"expected parseClientRoutes to return unresolved-route handling error",
				)
			}
			if !errors.Is(err, expectedErr) {
				t.Fatalf(
					"error = %v, expected unresolved-route handling error",
					err,
				)
			}
		},
	)
}

func TestResolveUnresolvedRoutePolicy(t *testing.T) {
	t.Run("returns required error when runtime is nil", func(t *testing.T) {
		_, err := resolveUnresolvedRoutePolicy(nil)
		if err == nil {
			t.Fatal("expected error when runtime is nil")
		}
		if !strings.Contains(err.Error(), "vorma runtime is required") {
			t.Fatalf("error = %q, expected nil-runtime message", err)
		}
	})

	t.Run("returns required error when config is nil", func(t *testing.T) {
		_, err := resolveUnresolvedRoutePolicy(
			&vormaruntime.Vorma{Log: testLogger()},
		)
		if err == nil {
			t.Fatal("expected error when config is nil")
		}
		if !strings.Contains(err.Error(), "vorma config is required") {
			t.Fatalf("error = %q, expected nil-config message", err)
		}
	})

	t.Run("defaults to error in production mode", func(t *testing.T) {
		v := &vormaruntime.Vorma{
			Config: mustParsedVormaConfigForRouteParseTests(t, &vormaruntime.VormaConfigJSON{}),
			Log:    testLogger(),
		}
		v.SetIsDev(false)

		policy, err := resolveUnresolvedRoutePolicy(v)
		if err != nil {
			t.Fatalf("resolveUnresolvedRoutePolicy returned error: %v", err)
		}
		if policy != vormaruntime.UnresolvedRoutePolicyError {
			t.Fatalf(
				"policy = %q, want %q",
				policy,
				vormaruntime.UnresolvedRoutePolicyError,
			)
		}
	})

	t.Run("defaults to error in dev mode", func(t *testing.T) {
		v := &vormaruntime.Vorma{
			Config: mustParsedVormaConfigForRouteParseTests(t, &vormaruntime.VormaConfigJSON{}),
			Log:    testLogger(),
		}
		v.SetIsDev(true)

		policy, err := resolveUnresolvedRoutePolicy(v)
		if err != nil {
			t.Fatalf("resolveUnresolvedRoutePolicy returned error: %v", err)
		}
		if policy != vormaruntime.UnresolvedRoutePolicyError {
			t.Fatalf(
				"policy = %q, want %q",
				policy,
				vormaruntime.UnresolvedRoutePolicyError,
			)
		}
	})

	t.Run("uses explicit config override", func(t *testing.T) {
		v := &vormaruntime.Vorma{
			Config: mustParsedVormaConfigForRouteParseTests(t, &vormaruntime.VormaConfigJSON{
				UnresolvedRoutePolicy: vormaruntime.UnresolvedRoutePolicyWarn,
			}),
			Log: testLogger(),
		}
		v.SetIsDev(false)

		policy, err := resolveUnresolvedRoutePolicy(v)
		if err != nil {
			t.Fatalf("resolveUnresolvedRoutePolicy returned error: %v", err)
		}
		if policy != vormaruntime.UnresolvedRoutePolicyWarn {
			t.Fatalf(
				"policy = %q, want %q",
				policy,
				vormaruntime.UnresolvedRoutePolicyWarn,
			)
		}
	})

	t.Run("returns parse error for unknown config override", func(t *testing.T) {
		rawConfig := defaultRawVormaConfigJSONForRouteParseTests()
		rawConfig.UnresolvedRoutePolicy = "unknown"
		_, parseError := parseVormaConfigForRouteParseTests(t, rawConfig)
		if parseError == nil {
			t.Fatal("expected parse error for unknown unresolved-route policy")
		}
		if !strings.Contains(
			parseError.Error(),
			"Vorma.UnresolvedRoutePolicy must be",
		) {
			t.Fatalf("error = %q, expected invalid-policy parse message", parseError)
		}
	})
}

func TestHandleUnresolvedRouteCalls(t *testing.T) {
	unresolvedRoutes := []unresolvedRouteCall{
		{
			Pattern:       "/dynamic",
			RawModuleExpr: "getPath(...)",
			Reason:        "cannot statically analyze",
		},
	}

	t.Run(
		"returns nil when warn policy is explicitly configured",
		func(t *testing.T) {
			v := &vormaruntime.Vorma{
				Config: mustParsedVormaConfigForRouteParseTests(t, &vormaruntime.VormaConfigJSON{
					UnresolvedRoutePolicy: vormaruntime.UnresolvedRoutePolicyWarn,
				}),
				Log: testLogger(),
			}
			v.SetIsDev(true)

			if err := handleUnresolvedRouteCalls(v, "frontend/src/vorma.routes.ts", unresolvedRoutes); err != nil {
				t.Fatalf("handleUnresolvedRouteCalls returned error: %v", err)
			}
		},
	)

	t.Run("returns detailed error for strict policy", func(t *testing.T) {
		v := &vormaruntime.Vorma{
			Config: mustParsedVormaConfigForRouteParseTests(t, &vormaruntime.VormaConfigJSON{}),
			Log:    testLogger(),
		}
		v.SetIsDev(false)

		err := handleUnresolvedRouteCalls(
			v,
			"frontend/src/vorma.routes.ts",
			unresolvedRoutes,
		)
		if err == nil {
			t.Fatal("expected strict unresolved-route policy to fail")
		}
		if !strings.Contains(
			err.Error(),
			`unresolved route calls are not allowed in "frontend/src/vorma.routes.ts"`,
		) {
			t.Fatalf("error = %q, expected route definition file context", err)
		}
		if !strings.Contains(err.Error(), `pattern "/dynamic"`) {
			t.Fatalf("error = %q, expected unresolved pattern details", err)
		}
	})
}

func TestParseRouteDefinitionsCodeIntoCalls(t *testing.T) {
	v := &vormaruntime.Vorma{Log: testLogger()}

	t.Run("transforms and extracts calls", func(t *testing.T) {
		executor := newRouteParsingExecutorForTest(
			func(dependencies *routeParsingExecutorDependencies) {
				dependencies.transformRouteDefinitionsCode = func(
					_ *vormaruntime.Vorma,
					code []byte,
				) (string, error) {
					if string(code) != "raw route defs" {
						t.Fatalf(
							"transform code = %q, want %q",
							string(code),
							"raw route defs",
						)
					}
					return "transformed route defs", nil
				}
				dependencies.extractRouteCallsFromTransformedCode = func(
					code string,
				) ([]routeCall, []unresolvedRouteCall, error) {
					if code != "transformed route defs" {
						t.Fatalf(
							"extract code = %q, want %q",
							code,
							"transformed route defs",
						)
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
			},
		)

		parsed, err := executor.parseRouteDefinitionsCodeIntoCalls(
			v,
			[]byte("raw route defs"),
		)
		if err != nil {
			t.Fatalf(
				"parseRouteDefinitionsCodeIntoCalls returned error: %v",
				err,
			)
		}
		if len(parsed.routeCalls) != 1 ||
			parsed.routeCalls[0].Pattern != "/home" {
			t.Fatalf(
				"route calls = %#v, want /home route call",
				parsed.routeCalls,
			)
		}
		if len(parsed.unresolvedRoutes) != 1 ||
			parsed.unresolvedRoutes[0].Pattern != "/dynamic" {
			t.Fatalf(
				"unresolved routes = %#v, want /dynamic unresolved route",
				parsed.unresolvedRoutes,
			)
		}
	})

	t.Run("returns transform error and skips extract", func(t *testing.T) {
		expectedErr := errors.New("transform failed")
		executor := newRouteParsingExecutorForTest(
			func(dependencies *routeParsingExecutorDependencies) {
				dependencies.transformRouteDefinitionsCode = func(
					*vormaruntime.Vorma,
					[]byte,
				) (string, error) {
					return "", expectedErr
				}
				dependencies.extractRouteCallsFromTransformedCode = func(
					string,
				) ([]routeCall, []unresolvedRouteCall, error) {
					t.Fatal("did not expect extract step after transform error")
					return nil, nil, nil
				}
			},
		)

		_, err := executor.parseRouteDefinitionsCodeIntoCalls(
			v,
			[]byte("raw route defs"),
		)
		if err == nil {
			t.Fatal(
				"expected parseRouteDefinitionsCodeIntoCalls to return transform error",
			)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected transform error", err)
		}
	})

	t.Run("wraps extract error", func(t *testing.T) {
		expectedErr := errors.New("extract failed")
		executor := newRouteParsingExecutorForTest(
			func(dependencies *routeParsingExecutorDependencies) {
				dependencies.transformRouteDefinitionsCode = func(
					*vormaruntime.Vorma,
					[]byte,
				) (string, error) {
					return "transformed route defs", nil
				}
				dependencies.extractRouteCallsFromTransformedCode = func(
					string,
				) ([]routeCall, []unresolvedRouteCall, error) {
					return nil, nil, expectedErr
				}
			},
		)

		_, err := executor.parseRouteDefinitionsCodeIntoCalls(
			v,
			[]byte("raw route defs"),
		)
		if err == nil {
			t.Fatal(
				"expected parseRouteDefinitionsCodeIntoCalls to return extract error",
			)
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

	_, err := parseRouteDefinitionFileIntoCalls(
		v,
		filepath.Join(t.TempDir(), "missing.vorma.routes.ts"),
	)
	if err == nil {
		t.Fatal("expected read error for missing route definition file")
	}
	if !strings.Contains(err.Error(), "read route definitions file") {
		t.Fatalf("error = %q, expected read-error context", err)
	}
}

func TestMergeRouteCallsIntoPaths(t *testing.T) {
	t.Run(
		"returns error when a route call has no module argument",
		func(t *testing.T) {
			v := &vormaruntime.Vorma{
				Config: mustParsedVormaConfigForRouteParseTests(t, &vormaruntime.VormaConfigJSON{
					ClientRouteDefinitionPatterns: []string{
						"frontend/src/**/*vorma.routes.ts",
					},
				}),
				Log: testLogger(),
			}

			paths := map[string]*vormaruntime.Path{}
			err := mergeRouteCallsIntoPaths(
				v,
				paths,
				"frontend/src/vorma.routes.ts",
				[]routeCall{{
					Pattern: "/missing-module",
					Module:  "",
					Key:     "default",
				}},
			)
			if err == nil {
				t.Fatal(
					"expected mergeRouteCallsIntoPaths to fail when module argument is missing",
				)
			}
			if !strings.Contains(
				err.Error(),
				"component module is required for pattern: /missing-module",
			) {
				t.Fatalf("error = %q, expected missing-module context", err)
			}
		},
	)

	t.Run("returns error when route pattern is duplicated", func(t *testing.T) {
		executor := newRouteParsingExecutorForTest(
			func(dependencies *routeParsingExecutorDependencies) {
				dependencies.computeRelativeModulePath = func(
					string,
					string,
				) (string, error) {
					return "frontend/src/routes/example.tsx", nil
				}
				dependencies.statRouteModulePath = func(
					string,
				) (fs.FileInfo, error) {
					return nil, nil
				}
			},
		)

		v := &vormaruntime.Vorma{
			Config: mustParsedVormaConfigForRouteParseTests(t, &vormaruntime.VormaConfigJSON{
				ClientRouteDefinitionPatterns: []string{
					"frontend/src/**/*vorma.routes.ts",
				},
			}),
			Log: testLogger(),
		}

		paths := map[string]*vormaruntime.Path{}
		err := executor.mergeRouteCallsIntoPaths(
			v,
			paths,
			"frontend/src/vorma.routes.ts",
			[]routeCall{
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
			},
		)
		if err == nil {
			t.Fatal(
				"expected mergeRouteCallsIntoPaths to fail on duplicate route pattern",
			)
		}
		if !strings.Contains(err.Error(), "duplicate route pattern: /dup") {
			t.Fatalf("error = %q, expected duplicate-pattern context", err)
		}
	})
}
