package vormabuild

import (
	"errors"
	"html/template"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/vormaruntime"
)

func TestGenerateAndAssembleTSContent_ErrorWrappingAndAssembly(t *testing.T) {
	restoreTSAssemblySteps := func(t *testing.T) {
		t.Helper()
		originalGenerateTypeScriptForTSAssembly := generatedTSAssemblyDeps.generateTypeScript
		originalGenerateRollupOptionsForTSAssembly := generatedTSAssemblyDeps.generateRollupInput
		originalGetEntrypointsForTSAssembly := generatedTSAssemblyDeps.getEntrypoints
		t.Cleanup(func() {
			generatedTSAssemblyDeps.generateTypeScript = originalGenerateTypeScriptForTSAssembly
			generatedTSAssemblyDeps.generateRollupInput = originalGenerateRollupOptionsForTSAssembly
			generatedTSAssemblyDeps.getEntrypoints = originalGetEntrypointsForTSAssembly
		})
	}

	t.Run("wraps generate TypeScript errors", func(t *testing.T) {
		restoreTSAssemblySteps(t)
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		expectedErr := errors.New("generate TS failed")
		generatedTSAssemblyDeps.generateTypeScript = func(tsGenInput) (string, error) {
			return "", expectedErr
		}
		generatedTSAssemblyDeps.generateRollupInput = func(*vormaruntime.LockedVorma, []string) (string, error) {
			t.Fatal("did not expect rollup options generation after TypeScript generation error")
			return "", nil
		}

		app.WithLock(func(l *vormaruntime.LockedVorma) {
			_, err := generateAndAssembleTSContent(app, l)
			if err == nil {
				t.Fatal("expected generateAndAssembleTSContent to return error")
			}
			if !strings.Contains(err.Error(), "generate TypeScript") {
				t.Fatalf("error = %q, expected generate-TypeScript context", err)
			}
			if !errors.Is(err, expectedErr) {
				t.Fatalf("error = %v, expected wrapped TypeScript generation error", err)
			}
		})
	})

	t.Run("wraps generate rollup options errors", func(t *testing.T) {
		restoreTSAssemblySteps(t)
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		generatedTSAssemblyDeps.generateTypeScript = func(tsGenInput) (string, error) {
			return "type A = 1;", nil
		}
		generatedTSAssemblyDeps.getEntrypoints = func(*vormaruntime.LockedVorma) []string {
			return []string{"frontend/src/vorma.entry.tsx"}
		}
		expectedErr := errors.New("rollup generation failed")
		generatedTSAssemblyDeps.generateRollupInput = func(*vormaruntime.LockedVorma, []string) (string, error) {
			return "", expectedErr
		}

		app.WithLock(func(l *vormaruntime.LockedVorma) {
			_, err := generateAndAssembleTSContent(app, l)
			if err == nil {
				t.Fatal("expected generateAndAssembleTSContent to return rollup error")
			}
			if !strings.Contains(err.Error(), "generate rollup options") {
				t.Fatalf("error = %q, expected generate-rollup-options context", err)
			}
			if !errors.Is(err, expectedErr) {
				t.Fatalf("error = %v, expected wrapped rollup generation error", err)
			}
		})
	})

	t.Run("concatenates TypeScript output and rollup options output", func(t *testing.T) {
		restoreTSAssemblySteps(t)
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		generatedTSAssemblyDeps.generateTypeScript = func(tsGenInput) (string, error) {
			return "TS_OUTPUT", nil
		}
		generatedTSAssemblyDeps.getEntrypoints = func(*vormaruntime.LockedVorma) []string {
			return []string{"frontend/src/vorma.entry.tsx"}
		}
		generatedTSAssemblyDeps.generateRollupInput = func(*vormaruntime.LockedVorma, []string) (string, error) {
			return "ROLLUP_OUTPUT", nil
		}

		app.WithLock(func(l *vormaruntime.LockedVorma) {
			contentBytes, err := generateAndAssembleTSContent(app, l)
			if err != nil {
				t.Fatalf("generateAndAssembleTSContent returned error: %v", err)
			}
			if string(contentBytes) != "TS_OUTPUTROLLUP_OUTPUT" {
				t.Fatalf("assembled output = %q, want %q", string(contentBytes), "TS_OUTPUTROLLUP_OUTPUT")
			}
		})
	})
}

func TestWriteGeneratedTS_DelegationAndErrors(t *testing.T) {
	restoreWriteGeneratedTSSteps := func(t *testing.T) {
		t.Helper()
		originalGenerateAndAssembleTSContentStep := generatedTSWriteDeps.generateAndAssembleTSContent
		originalWriteGeneratedTSContentIfChangedStep := generatedTSWriteDeps.writeGeneratedTSContentIfChanged
		t.Cleanup(func() {
			generatedTSWriteDeps.generateAndAssembleTSContent = originalGenerateAndAssembleTSContentStep
			generatedTSWriteDeps.writeGeneratedTSContentIfChanged = originalWriteGeneratedTSContentIfChangedStep
		})
	}

	t.Run("passes generated content to write step with expected target path", func(t *testing.T) {
		restoreWriteGeneratedTSSteps(t)
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		var writeCalled bool
		generatedTSWriteDeps.generateAndAssembleTSContent = func(*vormaruntime.Vorma, *vormaruntime.LockedVorma) ([]byte, error) {
			return []byte("GENERATED_CONTENT"), nil
		}
		generatedTSWriteDeps.writeGeneratedTSContentIfChanged = func(_ *vormaruntime.Vorma, targetPath string, contentBytes []byte) error {
			writeCalled = true
			expectedTargetPath := filepath.Join(".", app.Config.TSGenOutDir, "index.ts")
			if targetPath != expectedTargetPath {
				t.Fatalf("target path = %q, want %q", targetPath, expectedTargetPath)
			}
			if string(contentBytes) != "GENERATED_CONTENT" {
				t.Fatalf("content bytes = %q, want %q", string(contentBytes), "GENERATED_CONTENT")
			}
			return nil
		}

		app.WithLock(func(l *vormaruntime.LockedVorma) {
			if err := writeGeneratedTS(l); err != nil {
				t.Fatalf("writeGeneratedTS returned error: %v", err)
			}
		})
		if !writeCalled {
			t.Fatal("expected generatedTSWriteDeps.writeGeneratedTSContentIfChanged to be called")
		}
	})

	t.Run("returns assembly error", func(t *testing.T) {
		restoreWriteGeneratedTSSteps(t)
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		expectedErr := errors.New("assembly failed")
		generatedTSWriteDeps.generateAndAssembleTSContent = func(*vormaruntime.Vorma, *vormaruntime.LockedVorma) ([]byte, error) {
			return nil, expectedErr
		}
		generatedTSWriteDeps.writeGeneratedTSContentIfChanged = func(*vormaruntime.Vorma, string, []byte) error {
			t.Fatal("did not expect write step after assembly error")
			return nil
		}

		app.WithLock(func(l *vormaruntime.LockedVorma) {
			err := writeGeneratedTS(l)
			if err == nil {
				t.Fatal("expected writeGeneratedTS to return assembly error")
			}
			if !errors.Is(err, expectedErr) {
				t.Fatalf("error = %v, expected wrapped assembly error", err)
			}
		})
	})

	t.Run("returns write step error", func(t *testing.T) {
		restoreWriteGeneratedTSSteps(t)
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		expectedErr := errors.New("write failed")
		generatedTSWriteDeps.generateAndAssembleTSContent = func(*vormaruntime.Vorma, *vormaruntime.LockedVorma) ([]byte, error) {
			return []byte("ok"), nil
		}
		generatedTSWriteDeps.writeGeneratedTSContentIfChanged = func(*vormaruntime.Vorma, string, []byte) error {
			return expectedErr
		}

		app.WithLock(func(l *vormaruntime.LockedVorma) {
			err := writeGeneratedTS(l)
			if err == nil {
				t.Fatal("expected writeGeneratedTS to return write step error")
			}
			if !errors.Is(err, expectedErr) {
				t.Fatalf("error = %v, expected wrapped write step error", err)
			}
		})
	})
}

func TestWriteGeneratedTSContentIfChanged_ErrorBranches(t *testing.T) {
	restoreWriteGeneratedTSContentHelpers := func(t *testing.T) {
		t.Helper()
		originalGeneratedTSUnchangedStep := generatedTSWriteFileDeps.generatedTSUnchanged
		originalMakeGeneratedTSDirectory := generatedTSWriteFileDeps.makeGeneratedTSDirectory
		originalWriteGeneratedTSFile := generatedTSWriteFileDeps.writeGeneratedTSFile
		t.Cleanup(func() {
			generatedTSWriteFileDeps.generatedTSUnchanged = originalGeneratedTSUnchangedStep
			generatedTSWriteFileDeps.makeGeneratedTSDirectory = originalMakeGeneratedTSDirectory
			generatedTSWriteFileDeps.writeGeneratedTSFile = originalWriteGeneratedTSFile
		})
	}

	fixture := newBuildTestFixture(t, nil)
	app := fixture.app
	targetPath := filepath.Join(t.TempDir(), "generated", "index.ts")
	contentBytes := []byte("export const value = 1;")

	t.Run("wraps unchanged-check error", func(t *testing.T) {
		restoreWriteGeneratedTSContentHelpers(t)
		expectedErr := errors.New("unchanged check failed")
		generatedTSWriteFileDeps.generatedTSUnchanged = func(string, []byte) (bool, error) {
			return false, expectedErr
		}
		generatedTSWriteFileDeps.makeGeneratedTSDirectory = func(string, fs.FileMode) error {
			t.Fatal("did not expect directory creation after unchanged-check error")
			return nil
		}

		err := writeGeneratedTSContentIfChanged(app, targetPath, contentBytes)
		if err == nil {
			t.Fatal("expected writeGeneratedTSContentIfChanged to return unchanged-check error")
		}
		if !strings.Contains(err.Error(), "check existing generated file") {
			t.Fatalf("error = %q, expected unchanged-check context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped unchanged-check error", err)
		}
	})

	t.Run("skips write when content is unchanged", func(t *testing.T) {
		restoreWriteGeneratedTSContentHelpers(t)
		generatedTSWriteFileDeps.generatedTSUnchanged = func(string, []byte) (bool, error) {
			return true, nil
		}
		generatedTSWriteFileDeps.makeGeneratedTSDirectory = func(string, fs.FileMode) error {
			t.Fatal("did not expect directory creation when content is unchanged")
			return nil
		}
		generatedTSWriteFileDeps.writeGeneratedTSFile = func(string, []byte, fs.FileMode) error {
			t.Fatal("did not expect file write when content is unchanged")
			return nil
		}

		if err := writeGeneratedTSContentIfChanged(app, targetPath, contentBytes); err != nil {
			t.Fatalf("writeGeneratedTSContentIfChanged returned error: %v", err)
		}
	})

	t.Run("wraps create-directory error", func(t *testing.T) {
		restoreWriteGeneratedTSContentHelpers(t)
		expectedErr := errors.New("mkdir failed")
		generatedTSWriteFileDeps.generatedTSUnchanged = func(string, []byte) (bool, error) {
			return false, nil
		}
		generatedTSWriteFileDeps.makeGeneratedTSDirectory = func(string, fs.FileMode) error {
			return expectedErr
		}
		generatedTSWriteFileDeps.writeGeneratedTSFile = func(string, []byte, fs.FileMode) error {
			t.Fatal("did not expect file write after create-directory error")
			return nil
		}

		err := writeGeneratedTSContentIfChanged(app, targetPath, contentBytes)
		if err == nil {
			t.Fatal("expected writeGeneratedTSContentIfChanged to return create-directory error")
		}
		if !strings.Contains(err.Error(), "create directory") {
			t.Fatalf("error = %q, expected create-directory context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped create-directory error", err)
		}
	})

	t.Run("wraps write-file error", func(t *testing.T) {
		restoreWriteGeneratedTSContentHelpers(t)
		expectedErr := errors.New("write failed")
		generatedTSWriteFileDeps.generatedTSUnchanged = func(string, []byte) (bool, error) {
			return false, nil
		}
		generatedTSWriteFileDeps.makeGeneratedTSDirectory = func(string, fs.FileMode) error {
			return nil
		}
		generatedTSWriteFileDeps.writeGeneratedTSFile = func(string, []byte, fs.FileMode) error {
			return expectedErr
		}

		err := writeGeneratedTSContentIfChanged(app, targetPath, contentBytes)
		if err == nil {
			t.Fatal("expected writeGeneratedTSContentIfChanged to return write-file error")
		}
		if !strings.Contains(err.Error(), "write file") {
			t.Fatalf("error = %q, expected write-file context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped write-file error", err)
		}
	})

	t.Run("writes generated TS file with build artifact file mode", func(t *testing.T) {
		restoreWriteGeneratedTSContentHelpers(t)
		generatedTSWriteFileDeps.generatedTSUnchanged = func(string, []byte) (bool, error) {
			return false, nil
		}
		generatedTSWriteFileDeps.makeGeneratedTSDirectory = func(string, fs.FileMode) error {
			return nil
		}

		var capturedMode fs.FileMode
		generatedTSWriteFileDeps.writeGeneratedTSFile = func(_ string, _ []byte, fileMode fs.FileMode) error {
			capturedMode = fileMode
			return nil
		}

		if err := writeGeneratedTSContentIfChanged(app, targetPath, contentBytes); err != nil {
			t.Fatalf("writeGeneratedTSContentIfChanged returned error: %v", err)
		}
		if capturedMode != buildArtifactFileMode {
			t.Fatalf("file mode = %v, want %v", capturedMode, buildArtifactFileMode)
		}
	})
}

func TestRootDataTypeAlias(t *testing.T) {
	if got := rootDataTypeAlias(true); !strings.Contains(got, `type VormaRootData = Extract`) {
		t.Fatalf("rootDataTypeAlias(true) = %q, expected Extract type alias", got)
	}
	if got := rootDataTypeAlias(false); got != "type VormaRootData = null;" {
		t.Fatalf("rootDataTypeAlias(false) = %q, want %q", got, "type VormaRootData = null;")
	}
}

func TestBuildGeneratedTypeScriptBlock_AppendsExtraTSCodeAndNullRootData(t *testing.T) {
	loadersRouter := mux.NewNestedRouter(nil)
	actionsRouter := mux.NewRouter(&mux.Options{MountRoot: "/api/"})

	generatedCode := buildGeneratedTypeScriptBlock(
		tsGenInput{
			LoadersRouter: loadersRouter,
			ActionsRouter: actionsRouter,
			Config: &vormaruntime.VormaConfig{
				UIVariant: string(vormaruntime.UIVariantReact),
			},
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
		t.Fatalf("generated block missing null root data type alias:\n%s", generatedCode)
	}
	if !strings.Contains(generatedCode, `actionsRouterMountRoot: "/api/"`) {
		t.Fatalf("generated block missing actions mount root:\n%s", generatedCode)
	}
	if !strings.Contains(generatedCode, "export const extraCode = true;") {
		t.Fatalf("generated block missing appended extra TS code:\n%s", generatedCode)
	}
}

func TestRenderVitePluginConfig_ReturnsTemplateExecutionError(t *testing.T) {
	originalVitePluginTemplate := vitePluginTemplate
	t.Cleanup(func() {
		vitePluginTemplate = originalVitePluginTemplate
	})

	vitePluginTemplate = template.Must(template.New("broken").Parse(`{{index .Entrypoints 99}}`))
	_, err := renderVitePluginConfig(vitePluginTemplateData{
		Entrypoints: []string{"frontend/src/vorma.entry.tsx"},
	})
	if err == nil {
		t.Fatal("expected renderVitePluginConfig to return template execution error")
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

	vitePluginTemplate = template.Must(template.New("broken").Parse(`{{index .Entrypoints 99}}`))

	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		_, err := generateRollupOptions(l, []string{"frontend/src/vorma.entry.tsx"})
		if err == nil {
			t.Fatal("expected generateRollupOptions to return render error")
		}
		if !strings.Contains(err.Error(), "render vite plugin config") {
			t.Fatalf("error = %q, expected render-vite-plugin-config context", err)
		}
		if !strings.Contains(err.Error(), "error executing template") {
			t.Fatalf("error = %q, expected template execution context", err)
		}
	})
}
