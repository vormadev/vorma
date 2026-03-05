// This package is not meant to be called directly by users.
// If you are looking to bootstrap a new Vorma app, please
// run `npm create vorma@latest` in your terminal instead.
package bootstrap

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/kit/executil"
	"github.com/vormadev/vorma/kit/fsutil"
)

type Options struct {
	// e.g., "appname" or "modroot/apps/appname"
	GoImportBase string
	// "react", "preact", or "solid"
	UIVariant string
	// "npm", "pnpm", "yarn", or "bun"
	JSPackageManager string
	// "docker", "vercel", or "none"
	DeploymentTarget string
	IncludeTailwind  bool
	CreatedInDir     string // Empty if not created in a new directory
	NodeMajorVersion string // Example: "22"
	GoVersion        string // Example: "go1.24.0"

	// Monorepo support fields
	ModuleRoot      string // Absolute path to go.mod location
	CurrentDir      string // Absolute path where Vorma app is being created
	HasParentModule bool   // True if using a parent go.mod

	// SkipJavaScriptDependencyInstall avoids running package-manager install
	// commands for generated package dependencies.
	SkipJavaScriptDependencyInstall bool
	// SkipGoModTidy avoids running `go mod tidy` after scaffold generation.
	SkipGoModTidy bool
	// SkipInitialProjectBuild avoids running the first `go run ./backend/cmd/build`.
	SkipInitialProjectBuild bool
	// SuppressSuccessOutput avoids printing the interactive success banner.
	SuppressSuccessOutput bool
}

type derivedOptions struct {
	Options
	TSConfigJSXVal             string
	TSConfigJSXImportSourceVal string
	UIVitePlugin               string
	JSPackageManagerBaseCmd    string // "npx", "pnpm", "yarn", or "bunx"
	Call                       string
	PackageJSONExtras          string
	TailwindViteImport         string
	TailwindViteCall           string
	TailwindFileImport         string
	DynamicLinkParamsProp      string
	BackgroundColorKey         string
	StylePropOpen              string // "{{"
	StylePropClose             string // "}}"
	DockerLockFile             string
	DockerInstallCommand       string
	GoVersionForDocker         string

	/////// Monorepo Docker support
	// Path from app to module root (e.g., "../../")
	DockerBuildContextPath string
	// Path from module root to app (e.g., "apps/myapp")
	AppPathFromModuleRoot string
	// True if this is a monorepo setup
	IsMonorepo bool

	/////// Resolved Docker template fields
	// Either empty or newline + RUN command
	DockerPackageManagerInstall string
	// Either empty or newline + WORKDIR command
	DockerWorkdirCommand string
	// Path to binary in builder stage
	DockerBinaryPath string
}

const tw_vite_import = "import tailwindcss from \"@tailwindcss/vite\";\n"
const tw_vite_call = ", tailwindcss()"
const tw_file_import = "import \"./styles/tailwind.css\";\n"
const dynamic_link_params_prop = `{{ id: "42790214" }}`

type jsPackageManagerConfig struct {
	BaseCmd                     string
	RunScriptPrefix             string
	InstallCmd                  string
	DevDependencyCommand        string
	DevDependencyBaseArgs       []string
	DockerLockFile              string
	DockerInstallCommand        string
	DockerPackageManagerInstall string
}

var jsPackageManagerConfigByName = map[string]jsPackageManagerConfig{
	"npm": {
		BaseCmd:                     "npx",
		RunScriptPrefix:             "npm run",
		InstallCmd:                  "npm i",
		DevDependencyCommand:        "npm",
		DevDependencyBaseArgs:       []string{"i", "-D"},
		DockerLockFile:              "package-lock.json",
		DockerInstallCommand:        "npm ci",
		DockerPackageManagerInstall: "",
	},
	"pnpm": {
		BaseCmd:                     "pnpm",
		RunScriptPrefix:             "pnpm",
		InstallCmd:                  "pnpm i",
		DevDependencyCommand:        "pnpm",
		DevDependencyBaseArgs:       []string{"add", "-D"},
		DockerLockFile:              "pnpm-lock.yaml",
		DockerInstallCommand:        "pnpm i --frozen-lockfile",
		DockerPackageManagerInstall: "\nRUN npm i -g pnpm",
	},
	"yarn": {
		BaseCmd:                     "yarn",
		RunScriptPrefix:             "yarn",
		InstallCmd:                  "yarn",
		DevDependencyCommand:        "yarn",
		DevDependencyBaseArgs:       []string{"add", "-D"},
		DockerLockFile:              "yarn.lock",
		DockerInstallCommand:        "yarn install --frozen-lockfile",
		DockerPackageManagerInstall: "\nRUN npm i -g yarn",
	},
	"bun": {
		BaseCmd:                     "bunx",
		RunScriptPrefix:             "bun",
		InstallCmd:                  "bun i",
		DevDependencyCommand:        "bun",
		DevDependencyBaseArgs:       []string{"add", "-d"},
		DockerLockFile:              "bun.lockb",
		DockerInstallCommand:        "bun install --frozen-lockfile",
		DockerPackageManagerInstall: "\nRUN npm i -g bun",
	},
}

func mustGetJSPackageManagerConfig(
	jsPackageManager string,
) jsPackageManagerConfig {
	config, exists := jsPackageManagerConfigByName[jsPackageManager]
	if !exists {
		panic("unknown JSPackageManager: " + jsPackageManager)
	}
	return config
}

func (o Options) derived() derivedOptions {
	if o.UIVariant == "" {
		o.UIVariant = "react"
	}
	if o.JSPackageManager == "" {
		o.JSPackageManager = "npm"
	}

	do := derivedOptions{
		Options: o,
	}

	jsPackageManagerConfig := mustGetJSPackageManagerConfig(o.JSPackageManager)
	do.JSPackageManagerBaseCmd = jsPackageManagerConfig.BaseCmd

	do.BackgroundColorKey = "backgroundColor"

	switch o.UIVariant {
	case "react":
		do.TSConfigJSXVal = "react-jsx"
		do.TSConfigJSXImportSourceVal = "react"
	case "solid":
		do.TSConfigJSXVal = "preserve"
		do.TSConfigJSXImportSourceVal = "solid-js"
		do.Call = "()"
		do.BackgroundColorKey = `"background-color"`
	case "preact":
		do.TSConfigJSXVal = "react-jsx"
		do.TSConfigJSXImportSourceVal = "preact"
	}

	if o.DeploymentTarget != "none" &&
		o.DeploymentTarget != "vercel" &&
		o.DeploymentTarget != "docker" {
		panic("unknown DeploymentTarget: " + o.DeploymentTarget)
	}
	if o.DeploymentTarget == "docker" {
		if strings.TrimSpace(o.NodeMajorVersion) == "" {
			panic(
				"NodeMajorVersion must be set when DeploymentTarget is docker",
			)
		}
		if !isAllASCIIDigits(o.NodeMajorVersion) {
			panic("NodeMajorVersion must contain only digits")
		}
	}

	// Use Go version from options or fallback to runtime
	goVersion := o.GoVersion
	if goVersion == "" {
		goVersion = runtime.Version()
	}

	// Get Go version for Docker (format: 1.24)
	goVersionForDocker := strings.TrimPrefix(goVersion, "go")
	// Remove patch version for Docker tag (1.24.0 -> 1.24)
	if parts := strings.Split(goVersionForDocker, "."); len(parts) >= 2 {
		goVersionForDocker = parts[0] + "." + parts[1]
	}
	do.GoVersionForDocker = goVersionForDocker

	// Calculate monorepo paths for Docker
	if o.HasParentModule && o.ModuleRoot != "" && o.CurrentDir != "" {
		do.IsMonorepo = true

		// Calculate path from current dir back to module root
		relPath, err := filepath.Rel(o.CurrentDir, o.ModuleRoot)
		if err == nil {
			do.DockerBuildContextPath = filepath.ToSlash(relPath)
		}

		// Calculate path from module root to current dir
		appPath, err := filepath.Rel(o.ModuleRoot, o.CurrentDir)
		if err == nil {
			do.AppPathFromModuleRoot = filepath.ToSlash(appPath)
		}
	}

	if o.DeploymentTarget == "vercel" {
		do.PackageJSONExtras = fmt.Sprintf(
			`,
		"vercel-install-go": "curl -L https://go.dev/dl/%s.linux-amd64.tar.gz | tar -C /tmp -xz",
		"vercel-install": "%s vercel-install-go && %s",
		"vercel-build": "export PATH=/tmp/go/bin:$PATH && go run ./backend/cmd/build"`,
			goVersion,
			do.ResolveJSPackageManagerRunScriptPrefix(),
			do.ResolveJSPackageManagerInstallCmd(),
		)
	}

	if o.DeploymentTarget == "docker" {
		// Update package.json extras for Docker
		dockerBuildCmd := "docker build -t vorma-app ."
		if do.IsMonorepo && do.DockerBuildContextPath != "" {
			// For monorepo, build from module root
			dockerBuildCmd = fmt.Sprintf(
				"docker build -f Dockerfile -t vorma-app %s",
				do.DockerBuildContextPath,
			)
		}

		do.PackageJSONExtras = fmt.Sprintf(`,
		"docker-build": "%s",
		"docker-run": "docker run -d -p ${PORT:-8080}:${PORT:-8080} -e PORT=${PORT:-8080} vorma-app"`,
			dockerBuildCmd)

		do.DockerLockFile = jsPackageManagerConfig.DockerLockFile
		do.DockerInstallCommand = jsPackageManagerConfig.DockerInstallCommand
		do.DockerPackageManagerInstall = jsPackageManagerConfig.DockerPackageManagerInstall

		// Resolve Docker template fields
		if do.IsMonorepo {
			do.DockerWorkdirCommand = "\nWORKDIR /app/" + do.AppPathFromModuleRoot
			do.DockerBinaryPath = "/app/" + do.AppPathFromModuleRoot + "/backend/.wavedist/main"
		} else {
			do.DockerWorkdirCommand = "" // No extra WORKDIR needed
			do.DockerBinaryPath = "/app/backend/.wavedist/main"
		}
	}

	do.UIVitePlugin = resolveUIVitePlugin(do)

	do.DynamicLinkParamsProp = dynamic_link_params_prop

	do.StylePropOpen = "{{"
	do.StylePropClose = "}}"

	if o.IncludeTailwind {
		do.TailwindViteImport = tw_vite_import
		do.TailwindViteCall = tw_vite_call
		do.TailwindFileImport = tw_file_import
	}

	return do
}

var (
	//go:embed tmpls
	tmplsFS embed.FS
	//go:embed assets
	assetsFS embed.FS
)

func MustInit(o Options) {
	if o.GoImportBase == "" {
		panic("GoImportBase must be set")
	}

	do := o.derived()

	fsutil.EnsureDirs(
		"backend/assets",
		"frontend/assets",
		"backend/src/router",
		"backend/cmd/serve",
		"backend/cmd/build",
		"backend/.wavedist/static/internal",
		"frontend/src/components",
		"frontend/src/routes",
		"frontend/src/styles",
	)

	if o.DeploymentTarget == "vercel" {
		fsutil.EnsureDirs("api")
	}

	do.mustWriteTmpl(
		"backend/cmd/serve/main.go",
		"tmpls/cmd_app_main_go_tmpl.txt",
	)
	do.mustWriteTmpl(
		"backend/cmd/build/main.go",
		"tmpls/cmd_build_main_go_tmpl.txt",
	)
	do.mustWriteTmpl(
		"backend/.wavedist/static/.keep",
		"tmpls/dist_static_keep_tmpl.txt",
	)
	mustWriteStr(
		"backend/assets/entry.go.html",
		"tmpls/backend_static_entry_go_html_str.txt",
	)
	do.mustWriteTmpl(
		"backend/src/router/app.go",
		"tmpls/backend_src_router_app_go_tmpl.txt",
	)
	do.mustWriteTmpl(
		"backend/src/router/context.go",
		"tmpls/backend_src_router_context_go_tmpl.txt",
	)
	do.mustWriteTmpl(
		"backend/src/router/init.go",
		"tmpls/backend_src_router_init_go_tmpl.txt",
	)
	do.mustWriteTmpl(
		"backend/src/router/example_routes.go",
		"tmpls/backend_src_router_example_routes_go_tmpl.txt",
	)
	mustWriteStr("backend/wave.dev.go", "tmpls/backend_wave_dev_go_str.txt")
	mustWriteStr("backend/wave.prod.go", "tmpls/backend_wave_prod_go_str.txt")
	do.mustWriteTmpl(
		"backend/wave.config.json",
		"tmpls/wave_config_json_tmpl.txt",
	)
	do.mustWriteTmpl("vite.config.ts", "tmpls/vite_config_ts_tmpl.txt")
	do.mustWriteTmpl("package.json", "tmpls/package_json_tmpl.txt")
	mustWriteStr(".gitignore", "tmpls/gitignore_str.txt")
	mustWriteStr("frontend/src/styles/main.css", "tmpls/main_css_str.txt")
	mustWriteStr(
		"frontend/src/styles/main.critical.css",
		"tmpls/main_critical_css_str.txt",
	)
	mustWriteStr(
		"frontend/src/routes/core.vorma.routes.ts",
		"tmpls/frontend_routes_core_ts_str.txt",
	)
	mustWriteStr(
		"frontend/src/routes/links.vorma.routes.ts",
		"tmpls/frontend_routes_links_ts_str.txt",
	)
	do.mustWriteTmpl(
		"frontend/src/components/root.tsx",
		"tmpls/frontend_root_tsx_tmpl.txt",
	)
	do.mustWriteTmpl(
		"frontend/src/components/home.tsx",
		"tmpls/frontend_home_tsx_tmpl.txt",
	)
	do.mustWriteTmpl(
		"frontend/src/components/links.tsx",
		"tmpls/frontend_links_tsx_tmpl.txt",
	)
	do.mustWriteTmpl(
		"frontend/src/vorma.bindings.ts",
		"tmpls/frontend_bindings_ts_tmpl.txt",
	)
	mustWriteStr("frontend/vite.d.ts", "tmpls/frontend_vite_d_ts_str.txt")
	if o.DeploymentTarget == "vercel" {
		do.mustWriteTmpl("vercel.json", "tmpls/vercel_json_tmpl.txt")
		do.mustWriteTmpl("api/proxy.ts", "tmpls/api_proxy_ts_str.txt")
	}
	if o.DeploymentTarget == "docker" {
		do.mustWriteTmpl("Dockerfile", "tmpls/dockerfile_tmpl.txt")
	}

	// last
	do.mustWriteTmpl("tsconfig.json", "tmpls/ts_config_json_tmpl.txt")

	if !o.SkipJavaScriptDependencyInstall {
		mustInstallJSPkgs(
			do,
			"typescript",
			"vite",
			fmt.Sprintf("vorma@%s", vorma.CurrentReleaseVersion()),
			resolveUIVitePlugin(do),
		)
	}

	if do.UIVariant == "react" {
		do.mustWriteTmpl(
			"frontend/src/vorma.entry.tsx",
			"tmpls/frontend_entry_tsx_react_tmpl.txt",
		)

		if !o.SkipJavaScriptDependencyInstall {
			mustInstallJSPkgs(
				do,
				"react",
				"react-dom",
				"@types/react",
				"@types/react-dom",
			)
		}
	}

	if do.UIVariant == "solid" {
		do.mustWriteTmpl(
			"frontend/src/vorma.entry.tsx",
			"tmpls/frontend_entry_tsx_solid_tmpl.txt",
		)

		if !o.SkipJavaScriptDependencyInstall {
			mustInstallJSPkgs(do, "solid-js")
		}
	}

	if do.UIVariant == "preact" {
		do.mustWriteTmpl(
			"frontend/src/vorma.entry.tsx",
			"tmpls/frontend_entry_tsx_preact_tmpl.txt",
		)

		if !o.SkipJavaScriptDependencyInstall {
			mustInstallJSPkgs(do, "preact", "@preact/signals")
		}
	}

	if do.DeploymentTarget == "vercel" && !o.SkipJavaScriptDependencyInstall {
		mustInstallJSPkgs(do, "@vercel/node")
	}

	if do.IncludeTailwind {
		if !o.SkipJavaScriptDependencyInstall {
			mustInstallJSPkgs(do, "@tailwindcss/vite", "tailwindcss")
		}
		mustWriteStr(
			"frontend/src/styles/tailwind.css",
			"tmpls/frontend_css_tailwind_css_str.txt",
		)
	}

	// write assets
	mustWriteFile("frontend/assets/favicon.svg", "assets/favicon.svg")

	if !o.SkipGoModTidy {
		// tidy go modules
		if err := executil.RunCmd("go", "mod", "tidy"); err != nil {
			panic("failed to tidy go modules: " + err.Error())
		}
	}

	if !o.SkipInitialProjectBuild {
		// build once (no binary)
		if err := executil.RunCmd(
			"go",
			"run",
			"./backend/cmd/build",
			"--no-binary",
		); err != nil {
			panic("failed to run build command: " + err.Error())
		}
	}

	if !o.SuppressSuccessOutput {
		fmt.Println()
		fmt.Println("✨ SUCCESS! Your Vorma app is ready.")
		fmt.Println()
		if o.CreatedInDir != "" {
			fmt.Printf(
				"💻 Run `cd %s && %s dev` to start the development server.\n",
				o.CreatedInDir,
				do.ResolveJSPackageManagerRunScriptPrefix(),
			)
		} else {
			fmt.Printf("💻 Run `%s dev` to start the development server.\n",
				do.ResolveJSPackageManagerRunScriptPrefix(),
			)
		}
		fmt.Println()
	}
}

func (do derivedOptions) ResolveJSPackageManagerRunScriptPrefix() string {
	return mustGetJSPackageManagerConfig(do.JSPackageManager).RunScriptPrefix
}

func (do derivedOptions) ResolveJSPackageManagerInstallCmd() string {
	return mustGetJSPackageManagerConfig(do.JSPackageManager).InstallCmd
}

func resolveJSDevDependencyInstallCommand(
	jsPackageManager string,
	packages []string,
) (string, []string) {
	jsPackageManagerConfig := mustGetJSPackageManagerConfig(jsPackageManager)
	commandArguments := append(
		append([]string(nil), jsPackageManagerConfig.DevDependencyBaseArgs...),
		packages...,
	)
	return jsPackageManagerConfig.DevDependencyCommand, commandArguments
}

func mustInstallJSPkgs(do derivedOptions, packages ...string) {
	if len(packages) == 0 {
		return
	}

	command, commandArguments := resolveJSDevDependencyInstallCommand(
		do.JSPackageManager,
		packages,
	)
	fullCommand := append([]string{command}, commandArguments...)
	if err := executil.RunCmd(fullCommand...); err != nil {
		panic(
			"failed to install JS packages: " +
				strings.Join(packages, ", ") +
				": " +
				err.Error(),
		)
	}
}

func isAllASCIIDigits(value string) bool {
	for _, currentRune := range value {
		if currentRune < '0' || currentRune > '9' {
			return false
		}
	}
	return true
}

func resolveUIVitePlugin(do derivedOptions) string {
	switch do.UIVariant {
	case "":
		panic("UIVariant must be set")
	case "react":
		return "@vitejs/plugin-react-swc"
	case "solid":
		return "vite-plugin-solid"
	case "preact":
		return "@preact/preset-vite"
	}
	panic("unknown UI variant: " + do.UIVariant)
}

func (d *derivedOptions) mustWriteTmpl(target, name string) {
	tmplStr, err := tmplsFS.ReadFile(name)
	if err != nil {
		panic(err)
	}
	tmpl := template.Must(template.New(target).Parse(string(tmplStr)))
	var sb strings.Builder
	if err := tmpl.Execute(&sb, d); err != nil {
		panic(err)
	}
	b := []byte(sb.String())
	if err := os.WriteFile(target, b, 0644); err != nil {
		panic(err)
	}
}

func mustWriteStr(target, name string) {
	content, err := tmplsFS.ReadFile(name)
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(target, content, 0644); err != nil {
		panic(err)
	}
}

func mustWriteFile(target, source string) {
	b, err := assetsFS.ReadFile(source)
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(target, b, 0644); err != nil {
		panic(err)
	}
}
