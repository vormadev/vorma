package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

/////////////////////////////////////////////////////////////////////
/////// Types
/////////////////////////////////////////////////////////////////////

type commandOptions struct {
	outputDirectoryPath string
	repositoryRootPath  string
	uiAdapter           string
}

type overlayTemplateData struct {
	UIAdapter          string
	RepositoryRootPath string
}

/////////////////////////////////////////////////////////////////////
/////// Entry Point
/////////////////////////////////////////////////////////////////////

func main() {
	options, optionsError := parseCommandOptions()
	if optionsError != nil {
		fatalf("%v", optionsError)
	}

	if generateError := generateFixtureProject(options); generateError != nil {
		fatalf("%v", generateError)
	}
}

/////////////////////////////////////////////////////////////////////
/////// Generation
/////////////////////////////////////////////////////////////////////

func generateFixtureProject(options commandOptions) error {
	if makeDirectoryError := os.MkdirAll(options.outputDirectoryPath, 0755); makeDirectoryError != nil {
		return fmt.Errorf(
			"create output directory %q: %w",
			options.outputDirectoryPath,
			makeDirectoryError,
		)
	}

	if runBootstrapError := runBootstrapScaffoldGeneration(options); runBootstrapError != nil {
		return runBootstrapError
	}

	if removeDefaultFilesError := removeBootstrapDefaultAppFiles(options); removeDefaultFilesError != nil {
		return removeDefaultFilesError
	}

	overlayTemplatesRootPath := filepath.Join(
		options.repositoryRootPath,
		"internal",
		"e2e",
		"overlay_templates",
	)

	if applyCommonOverlayError := applyOverlayTemplateDirectory(
		applyOverlayTemplateDirectoryInput{
			templateDirectoryPath: filepath.Join(
				overlayTemplatesRootPath,
				"common",
			),
			outputDirectoryPath: options.outputDirectoryPath,
			overlayTemplateData: overlayTemplateData{
				UIAdapter:          options.uiAdapter,
				RepositoryRootPath: options.repositoryRootPath,
			},
		},
	); applyCommonOverlayError != nil {
		return applyCommonOverlayError
	}

	adapterOverlayDirectoryName := "react_like"
	if options.uiAdapter == "solid" {
		adapterOverlayDirectoryName = "solid"
	}

	if applyAdapterOverlayError := applyOverlayTemplateDirectory(
		applyOverlayTemplateDirectoryInput{
			templateDirectoryPath: filepath.Join(
				overlayTemplatesRootPath,
				adapterOverlayDirectoryName,
			),
			outputDirectoryPath: options.outputDirectoryPath,
			overlayTemplateData: overlayTemplateData{
				UIAdapter:          options.uiAdapter,
				RepositoryRootPath: options.repositoryRootPath,
			},
		},
	); applyAdapterOverlayError != nil {
		return applyAdapterOverlayError
	}

	if options.uiAdapter == "preact" {
		if applyPreactOverridesError := applyOverlayTemplateDirectory(
			applyOverlayTemplateDirectoryInput{
				templateDirectoryPath: filepath.Join(
					overlayTemplatesRootPath,
					"preact_overrides",
				),
				outputDirectoryPath: options.outputDirectoryPath,
				overlayTemplateData: overlayTemplateData{
					UIAdapter:          options.uiAdapter,
					RepositoryRootPath: options.repositoryRootPath,
				},
			},
		); applyPreactOverridesError != nil {
			return applyPreactOverridesError
		}
	}

	if ensureGoModuleDependenciesError := ensureFixtureGoModuleDependencies(options); ensureGoModuleDependenciesError != nil {
		return ensureGoModuleDependenciesError
	}
	if ensureJavaScriptDependenciesError := ensureFixtureJavaScriptDependencies(options); ensureJavaScriptDependenciesError != nil {
		return ensureJavaScriptDependenciesError
	}

	return nil
}

func runBootstrapScaffoldGeneration(options commandOptions) error {
	bootstrapRunnerFilePath, createTempFileError := writeBootstrapRunnerProgramFile(
		writeBootstrapRunnerProgramFileInput{
			repositoryRootPath:  options.repositoryRootPath,
			outputDirectoryPath: options.outputDirectoryPath,
			uiAdapter:           options.uiAdapter,
		},
	)
	if createTempFileError != nil {
		return createTempFileError
	}
	defer func() {
		_ = os.Remove(bootstrapRunnerFilePath)
	}()

	bootstrapRunCommand := exec.Command("go", "run", bootstrapRunnerFilePath)
	bootstrapRunCommand.Dir = options.repositoryRootPath
	bootstrapRunCommand.Env = append(
		os.Environ(),
		fmt.Sprintf("GOCACHE=%s", resolveGoCachePath()),
	)
	commandOutput, commandError := bootstrapRunCommand.CombinedOutput()
	if commandError != nil {
		return fmt.Errorf(
			"run bootstrap scaffold generator: %w\noutput:\n%s",
			commandError,
			string(commandOutput),
		)
	}
	return nil
}

func ensureFixtureGoModuleDependencies(options commandOptions) error {
	goModTidyCommand := exec.Command("go", "mod", "tidy")
	goModTidyCommand.Dir = options.outputDirectoryPath
	goModTidyCommand.Env = append(
		os.Environ(),
		fmt.Sprintf("GOCACHE=%s", resolveGoCachePath()),
	)
	commandOutput, commandError := goModTidyCommand.CombinedOutput()
	if commandError != nil {
		return fmt.Errorf(
			"run go mod tidy for generated fixture: %w\noutput:\n%s",
			commandError,
			string(commandOutput),
		)
	}

	return nil
}

func ensureFixtureJavaScriptDependencies(options commandOptions) error {
	pnpmInstallCommand := exec.Command("pnpm", "i")
	pnpmInstallCommand.Dir = options.outputDirectoryPath
	commandOutput, commandError := pnpmInstallCommand.CombinedOutput()
	if commandError != nil {
		return fmt.Errorf(
			"run pnpm install for generated fixture: %w\noutput:\n%s",
			commandError,
			string(commandOutput),
		)
	}

	return nil
}

type writeBootstrapRunnerProgramFileInput struct {
	repositoryRootPath  string
	outputDirectoryPath string
	uiAdapter           string
}

func writeBootstrapRunnerProgramFile(
	input writeBootstrapRunnerProgramFileInput,
) (string, error) {
	temporaryFileHandle, createTemporaryFileError := os.CreateTemp(
		"",
		"vorma-e2e-bootstrap-runner-*.go",
	)
	if createTemporaryFileError != nil {
		return "", fmt.Errorf(
			"create temporary bootstrap runner file: %w",
			createTemporaryFileError,
		)
	}
	defer temporaryFileHandle.Close()

	bootstrapProgramSourceCode := fmt.Sprintf(`package main

import (
	"os"

	"github.com/vormadev/vorma/bootstrap"
)

func main() {
	if err := os.Chdir("%s"); err != nil {
		panic(err)
	}
	bootstrap.MustInit(bootstrap.Options{
		GoImportBase: "%s",
		UIVariant: "%s",
		JSPackageManager: "pnpm",
		DeploymentTarget: "none",
		IncludeTailwind: false,
		ModuleRoot: "%s",
		CurrentDir: "%s",
		HasParentModule: false,
		SkipJavaScriptDependencyInstall: true,
		SkipGoModTidy: true,
		SkipInitialProjectBuild: true,
		SuppressSuccessOutput: true,
	})
}
`,
		input.outputDirectoryPath,
		"e2eapp",
		input.uiAdapter,
		input.outputDirectoryPath,
		input.outputDirectoryPath,
	)

	if _, writeError := temporaryFileHandle.WriteString(bootstrapProgramSourceCode); writeError != nil {
		return "", fmt.Errorf(
			"write temporary bootstrap runner file: %w",
			writeError,
		)
	}
	return temporaryFileHandle.Name(), nil
}

func removeBootstrapDefaultAppFiles(options commandOptions) error {
	defaultFileRelativePaths := []string{
		"backend/src/router/app.go",
		"backend/src/router/context.go",
		"backend/src/router/init.go",
		"backend/src/router/example_routes.go",
		"frontend/src/routes/links.vorma.routes.ts",
		"frontend/src/components/links.tsx",
	}

	for _, defaultFileRelativePath := range defaultFileRelativePaths {
		defaultFilePath := filepath.Join(
			options.outputDirectoryPath,
			filepath.FromSlash(defaultFileRelativePath),
		)
		if removeFileError := os.Remove(defaultFilePath); removeFileError != nil {
			if os.IsNotExist(removeFileError) {
				continue
			}
			return fmt.Errorf(
				"remove bootstrap default file %q: %w",
				defaultFilePath,
				removeFileError,
			)
		}
	}

	return nil
}

/////////////////////////////////////////////////////////////////////
/////// Overlay Templates
/////////////////////////////////////////////////////////////////////

type applyOverlayTemplateDirectoryInput struct {
	templateDirectoryPath string
	outputDirectoryPath   string
	overlayTemplateData   overlayTemplateData
}

func applyOverlayTemplateDirectory(
	input applyOverlayTemplateDirectoryInput,
) error {
	directoryInfo, statDirectoryError := os.Stat(input.templateDirectoryPath)
	if statDirectoryError != nil {
		if os.IsNotExist(statDirectoryError) {
			return nil
		}
		return fmt.Errorf(
			"stat overlay template directory %q: %w",
			input.templateDirectoryPath,
			statDirectoryError,
		)
	}
	if !directoryInfo.IsDir() {
		return fmt.Errorf(
			"overlay template directory path is not a directory: %q",
			input.templateDirectoryPath,
		)
	}

	walkError := filepath.WalkDir(
		input.templateDirectoryPath,
		func(currentPath string, directoryEntry fs.DirEntry, inputError error) error {
			if inputError != nil {
				return inputError
			}
			if directoryEntry.IsDir() {
				return nil
			}

			relativeTemplatePath, relativePathError := filepath.Rel(
				input.templateDirectoryPath,
				currentPath,
			)
			if relativePathError != nil {
				return fmt.Errorf(
					"resolve relative path for overlay template %q: %w",
					currentPath,
					relativePathError,
				)
			}

			targetRelativePath, templateFileMode, resolveTargetPathError :=
				resolveTargetRelativePathForTemplateFile(relativeTemplatePath)
			if resolveTargetPathError != nil {
				return resolveTargetPathError
			}

			targetAbsolutePath := filepath.Join(
				input.outputDirectoryPath,
				filepath.FromSlash(targetRelativePath),
			)
			if writeTemplateError := writeTemplateFileToOutput(
				writeTemplateFileToOutputInput{
					templateFilePath:    currentPath,
					targetFilePath:      targetAbsolutePath,
					templateFileMode:    templateFileMode,
					overlayTemplateData: input.overlayTemplateData,
				},
			); writeTemplateError != nil {
				return writeTemplateError
			}

			return nil
		},
	)
	if walkError != nil {
		return fmt.Errorf(
			"walk overlay template directory %q: %w",
			input.templateDirectoryPath,
			walkError,
		)
	}

	return nil
}

type templateFileMode int

const (
	templateFileModeCopy templateFileMode = iota
	templateFileModeRender
)

func resolveTargetRelativePathForTemplateFile(
	relativeTemplatePath string,
) (targetRelativePath string, mode templateFileMode, err error) {
	normalizedTemplatePath := filepath.ToSlash(relativeTemplatePath)
	switch {
	case strings.HasSuffix(normalizedTemplatePath, ".txt"):
		return strings.TrimSuffix(
			normalizedTemplatePath,
			".txt",
		), templateFileModeCopy, nil
	case strings.HasSuffix(normalizedTemplatePath, ".tmpl"):
		return strings.TrimSuffix(
			normalizedTemplatePath,
			".tmpl",
		), templateFileModeRender, nil
	default:
		return "", templateFileModeCopy, fmt.Errorf(
			"overlay template file %q must end with .txt or .tmpl",
			normalizedTemplatePath,
		)
	}
}

type writeTemplateFileToOutputInput struct {
	templateFilePath    string
	targetFilePath      string
	templateFileMode    templateFileMode
	overlayTemplateData overlayTemplateData
}

func writeTemplateFileToOutput(input writeTemplateFileToOutputInput) error {
	templateBytes, readTemplateError := os.ReadFile(input.templateFilePath)
	if readTemplateError != nil {
		return fmt.Errorf(
			"read overlay template file %q: %w",
			input.templateFilePath,
			readTemplateError,
		)
	}

	outputBytes := templateBytes
	if input.templateFileMode == templateFileModeRender {
		renderedTemplateBytes, renderTemplateError := renderTemplateBytes(
			renderTemplateBytesInput{
				templatePath:        input.templateFilePath,
				templateBytes:       templateBytes,
				overlayTemplateData: input.overlayTemplateData,
			},
		)
		if renderTemplateError != nil {
			return renderTemplateError
		}
		outputBytes = renderedTemplateBytes
	}

	if makeDirectoryError := os.MkdirAll(filepath.Dir(input.targetFilePath), 0755); makeDirectoryError != nil {
		return fmt.Errorf(
			"create parent directory for overlay output %q: %w",
			input.targetFilePath,
			makeDirectoryError,
		)
	}

	if writeOutputError := os.WriteFile(input.targetFilePath, outputBytes, 0644); writeOutputError != nil {
		return fmt.Errorf(
			"write overlay output file %q: %w",
			input.targetFilePath,
			writeOutputError,
		)
	}

	return nil
}

type renderTemplateBytesInput struct {
	templatePath        string
	templateBytes       []byte
	overlayTemplateData overlayTemplateData
}

func renderTemplateBytes(input renderTemplateBytesInput) ([]byte, error) {
	parsedTemplate, parseTemplateError := template.New(input.templatePath).
		Parse(
			string(input.templateBytes),
		)
	if parseTemplateError != nil {
		return nil, fmt.Errorf(
			"parse overlay template %q: %w",
			input.templatePath,
			parseTemplateError,
		)
	}

	var renderedOutput bytes.Buffer
	if executeTemplateError := parsedTemplate.Execute(
		&renderedOutput,
		input.overlayTemplateData,
	); executeTemplateError != nil {
		return nil, fmt.Errorf(
			"render overlay template %q: %w",
			input.templatePath,
			executeTemplateError,
		)
	}
	return renderedOutput.Bytes(), nil
}

/////////////////////////////////////////////////////////////////////
/////// CLI Parsing
/////////////////////////////////////////////////////////////////////

func parseCommandOptions() (commandOptions, error) {
	var options commandOptions
	flag.StringVar(
		&options.outputDirectoryPath,
		"output-dir",
		"",
		"target directory for generated fixture files",
	)
	flag.StringVar(
		&options.repositoryRootPath,
		"repository-root",
		"",
		"absolute repository root path",
	)
	flag.StringVar(
		&options.uiAdapter,
		"ui-adapter",
		"",
		"UI adapter for the generated fixture (solid|react|preact)",
	)
	flag.Parse()

	if flag.NArg() != 0 {
		return commandOptions{}, fmt.Errorf(
			"unexpected positional arguments: %s",
			strings.Join(flag.Args(), ", "),
		)
	}

	if strings.TrimSpace(options.outputDirectoryPath) == "" {
		return commandOptions{}, fmt.Errorf("--output-dir is required")
	}
	if strings.TrimSpace(options.repositoryRootPath) == "" {
		return commandOptions{}, fmt.Errorf("--repository-root is required")
	}
	if strings.TrimSpace(options.uiAdapter) == "" {
		return commandOptions{}, fmt.Errorf("--ui-adapter is required")
	}

	switch options.uiAdapter {
	case "solid", "react", "preact":
		// valid
	default:
		return commandOptions{}, fmt.Errorf(
			"--ui-adapter must be one of solid|react|preact, got %q",
			options.uiAdapter,
		)
	}

	absoluteOutputDirectoryPath, absoluteOutputDirectoryPathError := filepath.Abs(
		options.outputDirectoryPath,
	)
	if absoluteOutputDirectoryPathError != nil {
		return commandOptions{}, fmt.Errorf(
			"resolve --output-dir absolute path: %w",
			absoluteOutputDirectoryPathError,
		)
	}
	options.outputDirectoryPath = absoluteOutputDirectoryPath

	absoluteRepositoryRootPath, absoluteRepositoryRootPathError := filepath.Abs(
		options.repositoryRootPath,
	)
	if absoluteRepositoryRootPathError != nil {
		return commandOptions{}, fmt.Errorf(
			"resolve --repository-root absolute path: %w",
			absoluteRepositoryRootPathError,
		)
	}
	options.repositoryRootPath = absoluteRepositoryRootPath

	return options, nil
}

func resolveGoCachePath() string {
	if configuredGoCachePath := strings.TrimSpace(os.Getenv("GOCACHE")); configuredGoCachePath != "" {
		return configuredGoCachePath
	}
	return "/tmp/go-build"
}

func fatalf(format string, values ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", values...)
	os.Exit(2)
}
