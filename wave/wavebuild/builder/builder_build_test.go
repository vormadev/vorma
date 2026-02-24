package builder

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/vormadev/vorma/lab/jsonschema"
	"github.com/vormadev/vorma/wave"
)

func TestBuild_FileOnlyModeSkipsHooks(t *testing.T) {
	root := t.TempDir()
	config := newParsedConfigForBuilderBasicTestsAtRoot(root)
	config.Core.ServerOnlyMode = true

	markerPath := filepath.Join(root, "hook-marker.txt")
	config.Core.DevBuildHook = "printf 'user\\n' >> " + strconv.Quote(
		markerPath,
	)
	config.FrameworkDevBuildHook = "printf 'framework\\n' >> " + strconv.Quote(
		markerPath,
	)

	builderForTest := NewBuilder(
		config,
		newDiscardLoggerForBuilderBasicTests(),
	)
	defer builderForTest.Close()

	buildError := builderForTest.Build(BuildOpts{
		IsDev:        true,
		CompileGo:    false,
		IsRebuild:    false,
		FileOnlyMode: true,
	})
	if buildError != nil {
		t.Fatalf("Build(FileOnlyMode=true) returned error: %v", buildError)
	}

	if _, statError := os.Stat(markerPath); !os.IsNotExist(statError) {
		t.Fatalf(
			"expected hooks not to run in file-only mode, stat error: %v",
			statError,
		)
	}
}

func TestBuild_PropagatesHookFailure(t *testing.T) {
	config := newParsedConfigForBuilderBasicTestsAtRoot(t.TempDir())
	config.Core.ServerOnlyMode = true
	config.Core.DevBuildHook = "false"

	builderForTest := NewBuilder(
		config,
		newDiscardLoggerForBuilderBasicTests(),
	)
	defer builderForTest.Close()

	buildError := builderForTest.Build(BuildOpts{
		IsDev:     true,
		CompileGo: false,
		IsRebuild: false,
	})
	if buildError == nil {
		t.Fatal("expected build to fail when dev build hook fails")
	}
	if !strings.Contains(buildError.Error(), "build hook") {
		t.Fatalf("unexpected error: %v", buildError)
	}
}

func TestBuild_Success(t *testing.T) {
	config := newParsedConfigForBuilderBasicTestsAtRoot(t.TempDir())
	config.Core.ServerOnlyMode = true

	builderForTest := NewBuilder(
		config,
		newDiscardLoggerForBuilderBasicTests(),
	)
	defer builderForTest.Close()

	buildError := builderForTest.Build(BuildOpts{
		IsDev:     false,
		CompileGo: false,
		IsRebuild: false,
	})
	if buildError != nil {
		t.Fatalf("Build returned error: %v", buildError)
	}
}

func TestBuild_WritesConfigSchema(t *testing.T) {
	config := newParsedConfigForBuilderBasicTestsAtRoot(t.TempDir())
	config.Core.ServerOnlyMode = true

	builderForTest := NewBuilder(
		config,
		newDiscardLoggerForBuilderBasicTests(),
	)
	defer builderForTest.Close()

	if buildError := builderForTest.Build(BuildOpts{
		IsDev:     false,
		CompileGo: false,
		IsRebuild: false,
	}); buildError != nil {
		t.Fatalf("Build returned error: %v", buildError)
	}

	schemaPath := filepath.Join(config.Dist.Internal(), "schema.json")
	schemaBytes, readSchemaError := os.ReadFile(schemaPath)
	if readSchemaError != nil {
		t.Fatalf("read schema: %v", readSchemaError)
	}

	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if unmarshalError := json.Unmarshal(schemaBytes, &schema); unmarshalError != nil {
		t.Fatalf("unmarshal schema: %v", unmarshalError)
	}

	for _, requiredSection := range []string{"Core", "Vite", "Watch"} {
		if _, sectionExists := schema.Properties[requiredSection]; !sectionExists {
			t.Fatalf("schema missing %q section", requiredSection)
		}
	}
}

func TestBuild_IncludesRegisteredSchemaSection(t *testing.T) {
	config := newParsedConfigForBuilderBasicTestsAtRoot(t.TempDir())
	config.Core.ServerOnlyMode = true

	builderForTest := NewBuilder(
		config,
		newDiscardLoggerForBuilderBasicTests(),
	)
	defer builderForTest.Close()
	builderForTest.registerSchemaSection(
		"CustomFramework",
		jsonschema.OptionalObject(jsonschema.Def{
			Properties: struct {
				Enabled jsonschema.Entry
			}{
				Enabled: jsonschema.OptionalBoolean(
					jsonschema.Def{Default: true},
				),
			},
		}),
	)

	if buildError := builderForTest.Build(BuildOpts{
		IsDev:     false,
		CompileGo: false,
		IsRebuild: false,
	}); buildError != nil {
		t.Fatalf("Build returned error: %v", buildError)
	}

	schemaPath := filepath.Join(config.Dist.Internal(), "schema.json")
	schemaBytes, readSchemaError := os.ReadFile(schemaPath)
	if readSchemaError != nil {
		t.Fatalf("read schema: %v", readSchemaError)
	}

	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if unmarshalError := json.Unmarshal(schemaBytes, &schema); unmarshalError != nil {
		t.Fatalf("unmarshal schema: %v", unmarshalError)
	}

	if _, sectionExists := schema.Properties["CustomFramework"]; !sectionExists {
		t.Fatal("schema missing CustomFramework section")
	}
}

func TestBuild_CompileGoFailureIsReported(t *testing.T) {
	config := newParsedConfigForBuilderBasicTestsAtRoot(t.TempDir())
	config.Core.ServerOnlyMode = true
	config.Core.MainAppEntry = "this/package/does/not/exist"

	builderForTest := NewBuilder(
		config,
		newDiscardLoggerForBuilderBasicTests(),
	)
	defer builderForTest.Close()

	buildError := builderForTest.Build(BuildOpts{
		IsDev:     true,
		CompileGo: true,
		IsRebuild: false,
	})
	if buildError == nil {
		t.Fatal(
			"expected build to fail when Go compilation target does not exist",
		)
	}
	if !strings.Contains(buildError.Error(), "go compilation failed") {
		t.Fatalf("unexpected error: %v", buildError)
	}
}

func TestBuild_BrowserModeProcessesPublicFilesBeforeCSSBuild(t *testing.T) {
	root := t.TempDir()
	config := newParsedConfigForBuilderBasicTestsAtRoot(root)
	config.Core.ServerOnlyMode = false
	config.Core.CSSEntryFiles = cssEntryFilesForTests{
		NonCritical: filepath.Join(root, "styles", "main.css"),
	}
	config.Dist.Root = config.Core.DistDir

	if makeStylesDirectoryError := os.MkdirAll(
		filepath.Join(root, "styles"),
		0o755,
	); makeStylesDirectoryError != nil {
		t.Fatalf(
			"failed creating styles directory: %v",
			makeStylesDirectoryError,
		)
	}
	if writeMainCSSError := os.WriteFile(
		config.Core.CSSEntryFiles.NonCritical,
		[]byte(`.hero { background: url("images/logo.png"); }`),
		0o644,
	); writeMainCSSError != nil {
		t.Fatalf("failed writing main css: %v", writeMainCSSError)
	}

	publicAssetPath := filepath.Join(
		config.Core.StaticAssetDirs.Public,
		"images",
		"logo.png",
	)
	if makePublicAssetDirectoryError := os.MkdirAll(
		filepath.Dir(publicAssetPath),
		0o755,
	); makePublicAssetDirectoryError != nil {
		t.Fatalf(
			"failed creating public asset parent directory: %v",
			makePublicAssetDirectoryError,
		)
	}
	if writePublicAssetError := os.WriteFile(
		publicAssetPath,
		[]byte("logo"),
		0o644,
	); writePublicAssetError != nil {
		t.Fatalf("failed writing public asset file: %v", writePublicAssetError)
	}

	builderForTest := NewBuilder(
		config,
		newDiscardLoggerForBuilderBasicTests(),
	)
	defer builderForTest.Close()

	if buildError := builderForTest.Build(BuildOpts{
		IsDev:     false,
		CompileGo: false,
		IsRebuild: false,
	}); buildError != nil {
		t.Fatalf("Build returned error: %v", buildError)
	}

	normalRefBytes, readNormalRefError := os.ReadFile(
		config.Dist.NormalCSSRef(),
	)
	if readNormalRefError != nil {
		t.Fatalf("failed reading normal css ref: %v", readNormalRefError)
	}
	normalOutputPath := filepath.Join(
		config.Dist.StaticPublic(),
		strings.TrimSpace(string(normalRefBytes)),
	)
	normalOutputBytes, readNormalOutputError := os.ReadFile(normalOutputPath)
	if readNormalOutputError != nil {
		t.Fatalf(
			"failed reading generated normal css output: %v",
			readNormalOutputError,
		)
	}
	if !strings.Contains(string(normalOutputBytes), "vorma_out_images_logo_") {
		t.Fatalf(
			"expected generated css to reference hashed public asset, got:\n%s",
			string(normalOutputBytes),
		)
	}
}

func TestBuild_BrowserModeFailsOnPublicStaticCollisionBeforeCompileGo(
	t *testing.T,
) {
	root := t.TempDir()
	config := newParsedConfigForBuilderBasicTestsAtRoot(root)
	config.Core.ServerOnlyMode = false
	config.Core.MainAppEntry = "this/package/does/not/exist"
	config.Dist.Root = config.Core.DistDir

	collidingPublicFilePathA := filepath.Join(
		config.Core.StaticAssetDirs.Public,
		"logo.txt",
	)
	collidingPublicFilePathB := filepath.Join(
		config.Core.StaticAssetDirs.Public,
		wave.PrehashedDirname,
		"logo.txt",
	)
	if makePublicRootError := os.MkdirAll(
		filepath.Dir(collidingPublicFilePathB),
		0o755,
	); makePublicRootError != nil {
		t.Fatalf(
			"failed creating colliding public asset directories: %v",
			makePublicRootError,
		)
	}
	if writeCollisionAError := os.WriteFile(
		collidingPublicFilePathA,
		[]byte("a"),
		0o644,
	); writeCollisionAError != nil {
		t.Fatalf(
			"failed writing first colliding public asset: %v",
			writeCollisionAError,
		)
	}
	if writeCollisionBError := os.WriteFile(
		collidingPublicFilePathB,
		[]byte("b"),
		0o644,
	); writeCollisionBError != nil {
		t.Fatalf(
			"failed writing second colliding public asset: %v",
			writeCollisionBError,
		)
	}

	builderForTest := NewBuilder(
		config,
		newDiscardLoggerForBuilderBasicTests(),
	)
	defer builderForTest.Close()

	buildError := builderForTest.Build(BuildOpts{
		IsDev:     false,
		CompileGo: true,
		IsRebuild: false,
	})
	if buildError == nil {
		t.Fatal("expected Build to fail for colliding public static paths")
	}
	if !strings.Contains(buildError.Error(), "static source path collision") {
		t.Fatalf(
			"expected collision error before go compilation, got: %v",
			buildError,
		)
	}
	if strings.Contains(buildError.Error(), "go compilation failed") {
		t.Fatalf(
			"expected static processing failure to win before go compilation, got: %v",
			buildError,
		)
	}
}

func TestProcessFiles_FullCleanupPreservesOnlyWaveDevLockFile(t *testing.T) {
	config := newParsedConfigForBuilderBasicTestsAtRoot(t.TempDir())
	config.Core.ServerOnlyMode = true

	builderForTest := NewBuilder(
		config,
		newDiscardLoggerForBuilderBasicTests(),
	)
	defer builderForTest.Close()

	staticDirectoryPath := config.Dist.Static()
	if makeDirectoryError := os.MkdirAll(staticDirectoryPath, 0o755); makeDirectoryError != nil {
		t.Fatalf(
			"os.MkdirAll(%q) error = %v",
			staticDirectoryPath,
			makeDirectoryError,
		)
	}

	lockFilePath := filepath.Join(staticDirectoryPath, ".wave-dev.lock")
	if writeLockError := os.WriteFile(lockFilePath, []byte("123"), 0o644); writeLockError != nil {
		t.Fatalf("os.WriteFile(lock) error = %v", writeLockError)
	}

	nonLockWavePrefixedFilePath := filepath.Join(
		staticDirectoryPath,
		".wave-should-be-cleaned",
	)
	if writeNonLockWavePrefixedFileError := os.WriteFile(
		nonLockWavePrefixedFilePath,
		[]byte("stale"),
		0o644,
	); writeNonLockWavePrefixedFileError != nil {
		t.Fatalf(
			"os.WriteFile(non-lock wave-prefixed) error = %v",
			writeNonLockWavePrefixedFileError,
		)
	}

	regularFilePath := filepath.Join(staticDirectoryPath, "regular.txt")
	if writeRegularFileError := os.WriteFile(
		regularFilePath,
		[]byte("stale"),
		0o644,
	); writeRegularFileError != nil {
		t.Fatalf("os.WriteFile(regular) error = %v", writeRegularFileError)
	}

	if processError := builderForTest.processFiles(false, true); processError != nil {
		t.Fatalf("processFiles(false, true) error = %v", processError)
	}

	if _, statLockError := os.Stat(lockFilePath); statLockError != nil {
		t.Fatalf(
			"expected lock file to be preserved, stat error = %v",
			statLockError,
		)
	}
	if _, statNonLockWavePrefixedFileError := os.Stat(nonLockWavePrefixedFilePath); !os.IsNotExist(
		statNonLockWavePrefixedFileError,
	) {
		t.Fatalf(
			"expected non-lock .wave-* file to be removed, stat error = %v",
			statNonLockWavePrefixedFileError,
		)
	}
	if _, statRegularFileError := os.Stat(regularFilePath); !os.IsNotExist(
		statRegularFileError,
	) {
		t.Fatalf(
			"expected regular file to be removed, stat error = %v",
			statRegularFileError,
		)
	}
}

func TestWritePublicFileMapTS_WritesTypeScriptAndJSON(t *testing.T) {
	root := t.TempDir()
	config := newParsedConfigForBuilderBasicTestsAtRoot(root)
	config.Core.ServerOnlyMode = true

	publicAssetPath := filepath.Join(
		config.Core.StaticAssetDirs.Public,
		"assets",
		"logo.txt",
	)
	if mkdirError := os.MkdirAll(filepath.Dir(publicAssetPath), 0o755); mkdirError != nil {
		t.Fatalf("mkdir public asset dir: %v", mkdirError)
	}
	if writeError := os.WriteFile(publicAssetPath, []byte("logo"), 0o644); writeError != nil {
		t.Fatalf("write public asset: %v", writeError)
	}

	builderForTest := NewBuilder(
		config,
		newDiscardLoggerForBuilderBasicTests(),
	)
	defer builderForTest.Close()

	if processError := builderForTest.ProcessPublicFilesOnly(); processError != nil {
		t.Fatalf("process public files: %v", processError)
	}

	outDir := filepath.Join(root, "framework-map")
	if writeError := builderForTest.WritePublicFileMapTS(outDir); writeError != nil {
		t.Fatalf("WritePublicFileMapTS returned error: %v", writeError)
	}

	typeScriptPath := filepath.Join(outDir, wave.RelPaths.PublicFileMapTSName())
	if _, statError := os.Stat(typeScriptPath); statError != nil {
		t.Fatalf(
			"expected TS file at %q, stat error: %v",
			typeScriptPath,
			statError,
		)
	}

	jsonPath := filepath.Join(outDir, wave.RelPaths.PublicFileMapJSONName())
	jsonBytes, readJSONError := os.ReadFile(jsonPath)
	if readJSONError != nil {
		t.Fatalf("read JSON map file %q: %v", jsonPath, readJSONError)
	}
	if !strings.Contains(string(jsonBytes), "assets/logo.txt") {
		t.Fatalf(
			"expected JSON map to include source path key, got %q",
			string(jsonBytes),
		)
	}
}

func TestSavePublicFileMapJS_WritesHashedArtifactAndRef(t *testing.T) {
	root := t.TempDir()
	config := newParsedConfigForBuilderBasicTestsAtRoot(root)
	builderForTest := NewBuilder(
		config,
		newDiscardLoggerForBuilderBasicTests(),
	)
	defer builderForTest.Close()

	fileMap := wave.FileMap{
		"assets/logo.txt": {
			DistName:    "assets/logo_hashed.txt",
			ContentHash: "abc123",
		},
	}
	if saveError := builderForTest.savePublicFileMapJS(fileMap); saveError != nil {
		t.Fatalf("savePublicFileMapJS returned error: %v", saveError)
	}

	refBytes, readRefError := os.ReadFile(config.Dist.PublicFileMapRef())
	if readRefError != nil {
		t.Fatalf("read public file map ref: %v", readRefError)
	}
	hashedArtifactName := strings.TrimSpace(string(refBytes))
	if hashedArtifactName == "" {
		t.Fatal(
			"expected public file map ref file to contain hashed artifact filename",
		)
	}

	hashedArtifactPath := filepath.Join(
		config.Dist.StaticPublic(),
		hashedArtifactName,
	)
	if _, statError := os.Stat(hashedArtifactPath); statError != nil {
		t.Fatalf(
			"expected hashed public file map artifact at %q: %v",
			hashedArtifactPath,
			statError,
		)
	}
}
