package tsartifactgen

import (
	"errors"
	"html/template"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/nestedmux"
	"github.com/vormadev/vorma/vormabuild/internal/testkit"
)

func TestGenerateAndAssembleTSContent_ErrorWrappingAndAssembly(t *testing.T) {
	t.Run("wraps generate TypeScript errors", func(t *testing.T) {
		fixture := testkit.NewBuildTestFixture(t, nil)
		app := fixture.App

		expectedErr := errors.New("generate TS failed")
		dependencies := generatedTSAssemblyDependencies{
			generateTypeScript: func(tsGenInput) (string, error) {
				return "", expectedErr
			},
			generateRollupInput: func(*vormaruntime.LockedVorma, []string) (string, error) {
				t.Fatal(
					"did not expect rollup options generation after TypeScript generation error",
				)
				return "", nil
			},
		}

		app.WithLock(func(l *vormaruntime.LockedVorma) {
			_, err := generateAndAssembleTSContentWithDependencies(
				app,
				l,
				dependencies,
			)
			if err == nil {
				t.Fatal("expected generateAndAssembleTSContent to return error")
			}
			if !strings.Contains(err.Error(), "generate TypeScript") {
				t.Fatalf(
					"error = %q, expected generate-TypeScript context",
					err,
				)
			}
			if !errors.Is(err, expectedErr) {
				t.Fatalf(
					"error = %v, expected wrapped TypeScript generation error",
					err,
				)
			}
		})
	})

	t.Run("wraps generate rollup options errors", func(t *testing.T) {
		fixture := testkit.NewBuildTestFixture(t, nil)
		app := fixture.App

		expectedErr := errors.New("rollup generation failed")
		dependencies := generatedTSAssemblyDependencies{
			generateTypeScript: func(tsGenInput) (string, error) {
				return "type A = 1;", nil
			},
			getEntrypoints: func(*vormaruntime.LockedVorma) ([]string, error) {
				return []string{"frontend/src/vorma.entry.tsx"}, nil
			},
			generateRollupInput: func(*vormaruntime.LockedVorma, []string) (string, error) {
				return "", expectedErr
			},
		}

		app.WithLock(func(l *vormaruntime.LockedVorma) {
			_, err := generateAndAssembleTSContentWithDependencies(
				app,
				l,
				dependencies,
			)
			if err == nil {
				t.Fatal(
					"expected generateAndAssembleTSContent to return rollup error",
				)
			}
			if !strings.Contains(err.Error(), "generate rollup options") {
				t.Fatalf(
					"error = %q, expected generate-rollup-options context",
					err,
				)
			}
			if !errors.Is(err, expectedErr) {
				t.Fatalf(
					"error = %v, expected wrapped rollup generation error",
					err,
				)
			}
		})
	})

	t.Run(
		"concatenates TypeScript output and rollup options output",
		func(t *testing.T) {
			fixture := testkit.NewBuildTestFixture(t, nil)
			app := fixture.App

			dependencies := generatedTSAssemblyDependencies{
				generateTypeScript: func(tsGenInput) (string, error) {
					return "TS_OUTPUT", nil
				},
				getEntrypoints: func(*vormaruntime.LockedVorma) ([]string, error) {
					return []string{"frontend/src/vorma.entry.tsx"}, nil
				},
				generateRollupInput: func(*vormaruntime.LockedVorma, []string) (string, error) {
					return "ROLLUP_OUTPUT", nil
				},
			}

			app.WithLock(func(l *vormaruntime.LockedVorma) {
				contentBytes, err := generateAndAssembleTSContentWithDependencies(
					app,
					l,
					dependencies,
				)
				if err != nil {
					t.Fatalf(
						"generateAndAssembleTSContent returned error: %v",
						err,
					)
				}
				if string(contentBytes) != "TS_OUTPUTROLLUP_OUTPUT" {
					t.Fatalf(
						"assembled output = %q, want %q",
						string(contentBytes),
						"TS_OUTPUTROLLUP_OUTPUT",
					)
				}
			})
		},
	)
}

func TestGenerateAndAssembleTSContentForRouteBuildRuntimeStateSnapshot_ErrorWrappingAndAssembly(
	t *testing.T,
) {
	runtimeStateSnapshot := routeBuildRuntimeStateSnapshot{
		paths: map[string]*vormaruntime.Path{
			"/": {
				OriginalPattern: "/",
				SrcPath:         "frontend/src/routes/home.tsx",
				ExportKey:       "default",
			},
		},
		buildID: "snapshot-build-id",
	}

	t.Run("wraps generate TypeScript errors", func(t *testing.T) {
		fixture := testkit.NewBuildTestFixture(t, nil)
		app := fixture.App

		expectedErr := errors.New("generate TS failed")
		dependencies := generatedTSAssemblyDependencies{
			generateTypeScript: func(tsGenInput) (string, error) {
				return "", expectedErr
			},
			generateRollupInputForEntrypoints: func(*vormaruntime.Vorma, []string) (string, error) {
				t.Fatal(
					"did not expect rollup options generation after TypeScript generation error",
				)
				return "", nil
			},
		}

		_, err := generateAndAssembleTSContentForRouteBuildRuntimeStateSnapshotWithDependencies(
			app,
			runtimeStateSnapshot,
			dependencies,
		)
		if err == nil {
			t.Fatal(
				"expected generateAndAssembleTSContentForRouteBuildRuntimeState to return error",
			)
		}
		if !strings.Contains(err.Error(), "generate TypeScript") {
			t.Fatalf("error = %q, expected generate-TypeScript context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf(
				"error = %v, expected wrapped TypeScript generation error",
				err,
			)
		}
	})

	t.Run("wraps generate rollup options errors", func(t *testing.T) {
		fixture := testkit.NewBuildTestFixture(t, nil)
		app := fixture.App

		expectedErr := errors.New("rollup generation failed")
		dependencies := generatedTSAssemblyDependencies{
			generateTypeScript: func(tsGenInput) (string, error) {
				return "type A = 1;", nil
			},
			getEntrypointsForPaths: func(*vormaruntime.Vorma, map[string]*vormaruntime.Path) ([]string, error) {
				return []string{"frontend/src/vorma.entry.tsx"}, nil
			},
			generateRollupInputForEntrypoints: func(*vormaruntime.Vorma, []string) (string, error) {
				return "", expectedErr
			},
		}

		_, err := generateAndAssembleTSContentForRouteBuildRuntimeStateSnapshotWithDependencies(
			app,
			runtimeStateSnapshot,
			dependencies,
		)
		if err == nil {
			t.Fatal(
				"expected generateAndAssembleTSContentForRouteBuildRuntimeState to return rollup error",
			)
		}
		if !strings.Contains(err.Error(), "generate rollup options") {
			t.Fatalf(
				"error = %q, expected generate-rollup-options context",
				err,
			)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf(
				"error = %v, expected wrapped rollup generation error",
				err,
			)
		}
	})

	t.Run(
		"concatenates TypeScript output and rollup options output",
		func(t *testing.T) {
			fixture := testkit.NewBuildTestFixture(t, nil)
			app := fixture.App

			dependencies := generatedTSAssemblyDependencies{
				generateTypeScript: func(tsGenInput) (string, error) {
					return "TS_OUTPUT", nil
				},
				getEntrypointsForPaths: func(*vormaruntime.Vorma, map[string]*vormaruntime.Path) ([]string, error) {
					return []string{"frontend/src/vorma.entry.tsx"}, nil
				},
				generateRollupInputForEntrypoints: func(*vormaruntime.Vorma, []string) (string, error) {
					return "ROLLUP_OUTPUT", nil
				},
			}

			contentBytes, err := generateAndAssembleTSContentForRouteBuildRuntimeStateSnapshotWithDependencies(
				app,
				runtimeStateSnapshot,
				dependencies,
			)
			if err != nil {
				t.Fatalf(
					"generateAndAssembleTSContentForRouteBuildRuntimeState returned error: %v",
					err,
				)
			}
			if string(contentBytes) != "TS_OUTPUTROLLUP_OUTPUT" {
				t.Fatalf(
					"assembled output = %q, want %q",
					string(contentBytes),
					"TS_OUTPUTROLLUP_OUTPUT",
				)
			}
		},
	)
}

func TestWriteGeneratedTS_DelegationAndErrors(t *testing.T) {
	t.Run(
		"passes generated content to write step with expected target path",
		func(t *testing.T) {
			fixture := testkit.NewBuildTestFixture(t, nil)
			app := fixture.App

			var writeCalled bool
			dependencies := generatedTSWriteDependencies{
				generateAndAssembleTSContent: func(*vormaruntime.Vorma, *vormaruntime.LockedVorma) ([]byte, error) {
					return []byte("GENERATED_CONTENT"), nil
				},
				writeGeneratedTSContentIfChanged: func(
					_ *vormaruntime.Vorma,
					targetPath string,
					contentBytes []byte,
				) error {
					writeCalled = true
					expectedTargetPath := filepath.Join(
						app.Config.TSGenOutDir(),
						"index.ts",
					)
					if targetPath != expectedTargetPath {
						t.Fatalf(
							"target path = %q, want %q",
							targetPath,
							expectedTargetPath,
						)
					}
					if string(contentBytes) != "GENERATED_CONTENT" {
						t.Fatalf(
							"content bytes = %q, want %q",
							string(contentBytes),
							"GENERATED_CONTENT",
						)
					}
					return nil
				},
			}

			app.WithLock(func(l *vormaruntime.LockedVorma) {
				if err := writeGeneratedTSWithDependencies(l, dependencies); err != nil {
					t.Fatalf("writeGeneratedTS returned error: %v", err)
				}
			})
			if !writeCalled {
				t.Fatal(
					"expected writeGeneratedTSContentIfChanged to be called",
				)
			}
		},
	)

	t.Run("returns assembly error", func(t *testing.T) {
		fixture := testkit.NewBuildTestFixture(t, nil)
		app := fixture.App

		expectedErr := errors.New("assembly failed")
		dependencies := generatedTSWriteDependencies{
			generateAndAssembleTSContent: func(*vormaruntime.Vorma, *vormaruntime.LockedVorma) ([]byte, error) {
				return nil, expectedErr
			},
			writeGeneratedTSContentIfChanged: func(*vormaruntime.Vorma, string, []byte) error {
				t.Fatal("did not expect write step after assembly error")
				return nil
			},
		}

		app.WithLock(func(l *vormaruntime.LockedVorma) {
			err := writeGeneratedTSWithDependencies(l, dependencies)
			if err == nil {
				t.Fatal("expected writeGeneratedTS to return assembly error")
			}
			if !errors.Is(err, expectedErr) {
				t.Fatalf("error = %v, expected wrapped assembly error", err)
			}
		})
	})

	t.Run("returns write step error", func(t *testing.T) {
		fixture := testkit.NewBuildTestFixture(t, nil)
		app := fixture.App

		expectedErr := errors.New("write failed")
		dependencies := generatedTSWriteDependencies{
			generateAndAssembleTSContent: func(*vormaruntime.Vorma, *vormaruntime.LockedVorma) ([]byte, error) {
				return []byte("ok"), nil
			},
			writeGeneratedTSContentIfChanged: func(*vormaruntime.Vorma, string, []byte) error {
				return expectedErr
			},
		}

		app.WithLock(func(l *vormaruntime.LockedVorma) {
			err := writeGeneratedTSWithDependencies(l, dependencies)
			if err == nil {
				t.Fatal("expected writeGeneratedTS to return write step error")
			}
			if !errors.Is(err, expectedErr) {
				t.Fatalf("error = %v, expected wrapped write step error", err)
			}
		})
	})
}

func TestWriteGeneratedTSForRouteBuildRuntimeStateSnapshot_DelegationAndErrors(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	runtimeStateSnapshot := routeBuildRuntimeStateSnapshot{
		paths:   map[string]*vormaruntime.Path{},
		buildID: "snapshot-build-id",
	}

	t.Run(
		"passes generated content to write step with expected target path",
		func(t *testing.T) {
			var writeCalled bool
			dependencies := generatedTSWriteDependencies{
				generateAndAssembleTSContentForRuntimeStateSnapshot: func(
					_ *vormaruntime.Vorma,
					inputSnapshot routeBuildRuntimeStateSnapshot,
				) ([]byte, error) {
					if inputSnapshot.buildID != runtimeStateSnapshot.buildID {
						t.Fatalf(
							"runtime snapshot build ID = %q, want %q",
							inputSnapshot.buildID,
							runtimeStateSnapshot.buildID,
						)
					}
					return []byte("GENERATED_CONTENT"), nil
				},
				writeGeneratedTSContentIfChanged: func(
					_ *vormaruntime.Vorma,
					targetPath string,
					contentBytes []byte,
				) error {
					writeCalled = true
					expectedTargetPath := filepath.Join(
						app.Config.TSGenOutDir(),
						"index.ts",
					)
					if targetPath != expectedTargetPath {
						t.Fatalf(
							"target path = %q, want %q",
							targetPath,
							expectedTargetPath,
						)
					}
					if string(contentBytes) != "GENERATED_CONTENT" {
						t.Fatalf(
							"content bytes = %q, want %q",
							string(contentBytes),
							"GENERATED_CONTENT",
						)
					}
					return nil
				},
			}

			executor := newGeneratedTSWriteExecutor(dependencies)
			if err := executor.writeGeneratedTSForRouteBuildRuntimeState(app, runtimeStateSnapshot); err != nil {
				t.Fatalf(
					"writeGeneratedTSForRouteBuildRuntimeState returned error: %v",
					err,
				)
			}
			if !writeCalled {
				t.Fatal(
					"expected writeGeneratedTSContentIfChanged to be called",
				)
			}
		},
	)

	t.Run("returns assembly error", func(t *testing.T) {
		expectedErr := errors.New("assembly failed")
		dependencies := generatedTSWriteDependencies{
			generateAndAssembleTSContentForRuntimeStateSnapshot: func(
				*vormaruntime.Vorma,
				routeBuildRuntimeStateSnapshot,
			) ([]byte, error) {
				return nil, expectedErr
			},
			writeGeneratedTSContentIfChanged: func(*vormaruntime.Vorma, string, []byte) error {
				t.Fatal("did not expect write step after assembly error")
				return nil
			},
		}

		executor := newGeneratedTSWriteExecutor(dependencies)
		err := executor.writeGeneratedTSForRouteBuildRuntimeState(
			app,
			runtimeStateSnapshot,
		)
		if err == nil {
			t.Fatal(
				"expected writeGeneratedTSForRouteBuildRuntimeState to return assembly error",
			)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped assembly error", err)
		}
	})

	t.Run("returns write step error", func(t *testing.T) {
		expectedErr := errors.New("write failed")
		dependencies := generatedTSWriteDependencies{
			generateAndAssembleTSContentForRuntimeStateSnapshot: func(
				*vormaruntime.Vorma,
				routeBuildRuntimeStateSnapshot,
			) ([]byte, error) {
				return []byte("ok"), nil
			},
			writeGeneratedTSContentIfChanged: func(*vormaruntime.Vorma, string, []byte) error {
				return expectedErr
			},
		}

		executor := newGeneratedTSWriteExecutor(dependencies)
		err := executor.writeGeneratedTSForRouteBuildRuntimeState(
			app,
			runtimeStateSnapshot,
		)
		if err == nil {
			t.Fatal(
				"expected writeGeneratedTSForRouteBuildRuntimeState to return write step error",
			)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped write step error", err)
		}
	})
}

func TestWriteGeneratedTSContentIfChanged_ErrorBranches(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	targetPath := filepath.Join(t.TempDir(), "generated", "index.ts")
	contentBytes := []byte("export const value = 1;")

	t.Run("wraps unchanged-check error", func(t *testing.T) {
		expectedErr := errors.New("unchanged check failed")
		dependencies := generatedTSWriteFileDependencies{
			generatedTSUnchanged: func(string, []byte) (bool, error) {
				return false, expectedErr
			},
			makeGeneratedTSDirectory: func(string, fs.FileMode) error {
				t.Fatal(
					"did not expect directory creation after unchanged-check error",
				)
				return nil
			},
		}

		err := writeGeneratedTSContentIfChangedWithDependencies(
			app,
			targetPath,
			contentBytes,
			dependencies,
		)
		if err == nil {
			t.Fatal(
				"expected writeGeneratedTSContentIfChanged to return unchanged-check error",
			)
		}
		if !strings.Contains(err.Error(), "check existing generated file") {
			t.Fatalf("error = %q, expected unchanged-check context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped unchanged-check error", err)
		}
	})

	t.Run("skips write when content is unchanged", func(t *testing.T) {
		dependencies := generatedTSWriteFileDependencies{
			generatedTSUnchanged: func(string, []byte) (bool, error) {
				return true, nil
			},
			makeGeneratedTSDirectory: func(string, fs.FileMode) error {
				t.Fatal(
					"did not expect directory creation when content is unchanged",
				)
				return nil
			},
			writeGeneratedTSFile: func(string, []byte, fs.FileMode) error {
				t.Fatal("did not expect file write when content is unchanged")
				return nil
			},
		}

		if err := writeGeneratedTSContentIfChangedWithDependencies(
			app,
			targetPath,
			contentBytes,
			dependencies,
		); err != nil {
			t.Fatalf("writeGeneratedTSContentIfChanged returned error: %v", err)
		}
	})

	t.Run("wraps create-directory error", func(t *testing.T) {
		expectedErr := errors.New("mkdir failed")
		dependencies := generatedTSWriteFileDependencies{
			generatedTSUnchanged: func(string, []byte) (bool, error) {
				return false, nil
			},
			makeGeneratedTSDirectory: func(string, fs.FileMode) error {
				return expectedErr
			},
			writeGeneratedTSFile: func(string, []byte, fs.FileMode) error {
				t.Fatal(
					"did not expect file write after create-directory error",
				)
				return nil
			},
		}

		err := writeGeneratedTSContentIfChangedWithDependencies(
			app,
			targetPath,
			contentBytes,
			dependencies,
		)
		if err == nil {
			t.Fatal(
				"expected writeGeneratedTSContentIfChanged to return create-directory error",
			)
		}
		if !strings.Contains(err.Error(), "create directory") {
			t.Fatalf("error = %q, expected create-directory context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped create-directory error", err)
		}
	})

	t.Run("wraps write-file error", func(t *testing.T) {
		expectedErr := errors.New("write failed")
		dependencies := generatedTSWriteFileDependencies{
			generatedTSUnchanged: func(string, []byte) (bool, error) {
				return false, nil
			},
			makeGeneratedTSDirectory: func(string, fs.FileMode) error {
				return nil
			},
			writeGeneratedTSFile: func(string, []byte, fs.FileMode) error {
				return expectedErr
			},
		}

		err := writeGeneratedTSContentIfChangedWithDependencies(
			app,
			targetPath,
			contentBytes,
			dependencies,
		)
		if err == nil {
			t.Fatal(
				"expected writeGeneratedTSContentIfChanged to return write-file error",
			)
		}
		if !strings.Contains(err.Error(), "write file") {
			t.Fatalf("error = %q, expected write-file context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped write-file error", err)
		}
	})

	t.Run(
		"writes generated TS file with build artifact file mode",
		func(t *testing.T) {
			var capturedDirectoryMode fs.FileMode
			var capturedFileMode fs.FileMode
			dependencies := generatedTSWriteFileDependencies{
				generatedTSUnchanged: func(string, []byte) (bool, error) {
					return false, nil
				},
				makeGeneratedTSDirectory: func(
					_ string,
					fileMode fs.FileMode,
				) error {
					capturedDirectoryMode = fileMode
					return nil
				},
				writeGeneratedTSFile: func(_ string, _ []byte, fileMode fs.FileMode) error {
					capturedFileMode = fileMode
					return nil
				},
			}

			if err := writeGeneratedTSContentIfChangedWithDependencies(
				app,
				targetPath,
				contentBytes,
				dependencies,
			); err != nil {
				t.Fatalf(
					"writeGeneratedTSContentIfChanged returned error: %v",
					err,
				)
			}
			if capturedDirectoryMode != buildArtifactDirectoryMode {
				t.Fatalf(
					"directory mode = %v, want %v",
					capturedDirectoryMode,
					buildArtifactDirectoryMode,
				)
			}
			if capturedFileMode != buildArtifactFileMode {
				t.Fatalf(
					"file mode = %v, want %v",
					capturedFileMode,
					buildArtifactFileMode,
				)
			}
		},
	)
}

func TestRootDataTypeAlias(t *testing.T) {
	if got := rootDataTypeAlias(true); !strings.Contains(
		got,
		`type VormaRootData = Extract`,
	) {
		t.Fatalf(
			"rootDataTypeAlias(true) = %q, expected Extract type alias",
			got,
		)
	}
	if got := rootDataTypeAlias(false); got != "type VormaRootData = null;" {
		t.Fatalf(
			"rootDataTypeAlias(false) = %q, want %q",
			got,
			"type VormaRootData = null;",
		)
	}
}

func TestBuildGeneratedTypeScriptBlock_AppendsExtraTSCodeAndNullRootData(
	t *testing.T,
) {
	loadersRouter := nestedmux.NewRouter(nil)
	actionsRouter := mux.NewRouter(&mux.Options{MountRoot: "/api/"})

	generatedCode := buildGeneratedTypeScriptBlock(
		tsGenInput{
			LoadersRouter: loadersRouter,
			ActionsRouter: actionsRouter,
			Config: mustParsedVormaConfigForTSArtifactGenerationTests(
				t,
				&vormaruntime.VormaConfigJSON{
					UIVariant: string(vormaruntime.UIVariantReact),
				},
			),
			ExtraTSCode: "export const extraCode = true;",
		},
		false,
		routePatternMetadataConfig{
			actionsDynamicRune: ':',
			actionsSplatRune:   '*',
			loadersDynamicRune: ':',
			loadersSplatRune:   '*',
		},
	)

	if !strings.Contains(generatedCode, "type VormaRootData = null;") {
		t.Fatalf(
			"generated block missing null root data type alias:\n%s",
			generatedCode,
		)
	}
	if !strings.Contains(generatedCode, `actionsRouterMountRoot: "/api/"`) {
		t.Fatalf(
			"generated block missing actions mount root:\n%s",
			generatedCode,
		)
	}
	if !strings.Contains(generatedCode, "export const extraCode = true;") {
		t.Fatalf(
			"generated block missing appended extra TS code:\n%s",
			generatedCode,
		)
	}
}

func TestRenderVitePluginConfig_ReturnsTemplateExecutionError(t *testing.T) {
	originalVitePluginTemplate := vitePluginTemplate
	t.Cleanup(func() {
		vitePluginTemplate = originalVitePluginTemplate
	})

	vitePluginTemplate = template.Must(
		template.New("broken").Parse(`{{index .Entrypoints 99}}`),
	)
	_, err := renderVitePluginConfig(vitePluginTemplateData{
		Entrypoints: []string{"frontend/src/vorma.entry.tsx"},
	})
	if err == nil {
		t.Fatal(
			"expected renderVitePluginConfig to return template execution error",
		)
	}
	if !strings.Contains(err.Error(), "error executing template") {
		t.Fatalf("error = %q, expected template execution context", err)
	}
}

func TestGenerateRollupOptions_WrapsRenderError(t *testing.T) {
	originalVitePluginTemplate := vitePluginTemplate
	t.Cleanup(func() {
		vitePluginTemplate = originalVitePluginTemplate
	})

	vitePluginTemplate = template.Must(
		template.New("broken").Parse(`{{index .Entrypoints 99}}`),
	)

	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		_, err := generateRollupOptions(
			l,
			[]string{"frontend/src/vorma.entry.tsx"},
		)
		if err == nil {
			t.Fatal("expected generateRollupOptions to return render error")
		}
		if !strings.Contains(err.Error(), "render vite plugin config") {
			t.Fatalf(
				"error = %q, expected render-vite-plugin-config context",
				err,
			)
		}
		if !strings.Contains(err.Error(), "error executing template") {
			t.Fatalf("error = %q, expected template execution context", err)
		}
	})
}
