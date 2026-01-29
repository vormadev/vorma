// This package is not meant to be called directly by users.
// If you are looking to bootstrap a new Vorma app, please
// run `npm create vorma@latest` in your terminal instead.
package bootstrap

import (
	"embed"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

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

	switch o.JSPackageManager {
	case "npm":
		do.JSPackageManagerBaseCmd = "npx"
	case "pnpm":
		do.JSPackageManagerBaseCmd = "pnpm"
	case "yarn":
		do.JSPackageManagerBaseCmd = "yarn"
	case "bun":
		do.JSPackageManagerBaseCmd = "bunx"
	}

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
		do.PackageJSONExtras = fmt.Sprintf(`,
		"vercel-install-go": "curl -L https://go.dev/dl/%s.linux-amd64.tar.gz | tar -C /tmp -xz",
		"vercel-install": "%s vercel-install-go && %s",
		"vercel-build": "export PATH=/tmp/go/bin:$PATH && go run ./backend/cmd/build"`,
			goVersion, do.ResolveJSPackageManagerRunScriptPrefix(), do.ResolveJSPackageManagerInstallCmd(),
		)
	}

	if o.DeploymentTarget == "docker" {
		// Update package.json extras for Docker
		dockerBuildCmd := "docker build -t vorma-app ."
		if do.IsMonorepo && do.DockerBuildContextPath != "" {
			// For monorepo, build from module root
			dockerBuildCmd = fmt.Sprintf("docker build -f Dockerfile -t vorma-app %s", do.DockerBuildContextPath)
		}

		do.PackageJSONExtras = fmt.Sprintf(`,
		"docker-build": "%s",
		"docker-run": "docker run -d -p ${PORT:-8080}:${PORT:-8080} -e PORT=${PORT:-8080} vorma-app"`,
			dockerBuildCmd)

		// Set lock file and install command based on package manager
		switch o.JSPackageManager {
		case "npm":
			do.DockerLockFile = "package-lock.json"
			do.DockerInstallCommand = "npm ci"
			do.DockerPackageManagerInstall = ""
		case "pnpm":
			do.DockerLockFile = "pnpm-lock.yaml"
			do.DockerInstallCommand = "pnpm i --frozen-lockfile"
			do.DockerPackageManagerInstall = "\nRUN npm i -g pnpm"
		case "yarn":
			do.DockerLockFile = "yarn.lock"
			do.DockerInstallCommand = "yarn install --frozen-lockfile"
			do.DockerPackageManagerInstall = "\nRUN npm i -g yarn"
		case "bun":
			do.DockerLockFile = "bun.lockb"
			do.DockerInstallCommand = "bun install --frozen-lockfile"
			do.DockerPackageManagerInstall = "\nRUN npm i -g bun"
		}

		// Resolve Docker template fields
		if do.IsMonorepo {
			do.DockerWorkdirCommand = "\nWORKDIR /app/" + do.AppPathFromModuleRoot
			do.DockerBinaryPath = "/app/" + do.AppPathFromModuleRoot + "/backend/dist/main"
		} else {
			do.DockerWorkdirCommand = "" // No extra WORKDIR needed
			do.DockerBinaryPath = "/app/backend/dist/main"
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

func Init(o Options) {
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
		"backend/dist/static/internal",
		"frontend/src/components",
		"frontend/src/styles",
	)

	if o.DeploymentTarget == "vercel" {
		fsutil.EnsureDirs("api")
	}

	do.tmplWriteMust("backend/cmd/serve/main.go", "tmpls/cmd_app_main_go_tmpl.txt")
	do.tmplWriteMust("backend/cmd/build/main.go", "tmpls/cmd_build_main_go_tmpl.txt")
	do.tmplWriteMust("backend/dist/static/.keep", "tmpls/dist_static_keep_tmpl.txt")
	strWriteMust("backend/assets/entry.go.html", "tmpls/backend_static_entry_go_html_str.txt")
	do.tmplWriteMust("backend/src/router/router.go", "tmpls/backend_src_router_router_go_tmpl.txt")
	strWriteMust("backend/wave.dev.go", "tmpls/backend_wave_dev_go_str.txt")
	strWriteMust("backend/wave.prod.go", "tmpls/backend_wave_prod_go_str.txt")
	do.tmplWriteMust("backend/wave.config.json", "tmpls/wave_config_json_tmpl.txt")
	do.tmplWriteMust("vite.config.ts", "tmpls/vite_config_ts_tmpl.txt")
	do.tmplWriteMust("package.json", "tmpls/package_json_tmpl.txt")
	strWriteMust(".gitignore", "tmpls/gitignore_str.txt")
	strWriteMust("frontend/src/styles/main.css", "tmpls/main_css_str.txt")
	strWriteMust("frontend/src/styles/main.critical.css", "tmpls/main_critical_css_str.txt")
	strWriteMust("frontend/src/vorma.routes.ts", "tmpls/frontend_routes_ts_str.txt")
	do.tmplWriteMust("frontend/src/components/root.tsx", "tmpls/frontend_root_tsx_tmpl.txt")
	do.tmplWriteMust("frontend/src/components/home.tsx", "tmpls/frontend_home_tsx_tmpl.txt")
	do.tmplWriteMust("frontend/src/components/links.tsx", "tmpls/frontend_links_tsx_tmpl.txt")
	do.tmplWriteMust("frontend/src/vorma.utils.tsx", "tmpls/frontend_app_utils_tsx_tmpl.txt")
	strWriteMust("frontend/src/vorma.api.ts", "tmpls/frontend_api_client_ts_str.txt")
	strWriteMust("frontend/vite.d.ts", "tmpls/frontend_vite_d_ts_str.txt")
	if o.DeploymentTarget == "vercel" {
		do.tmplWriteMust("vercel.json", "tmpls/vercel_json_tmpl.txt")
		do.tmplWriteMust("api/proxy.ts", "tmpls/api_proxy_ts_str.txt")
	}
	if o.DeploymentTarget == "docker" {
		do.tmplWriteMust("Dockerfile", "tmpls/dockerfile_tmpl.txt")
	}

	// last
	do.tmplWriteMust("tsconfig.json", "tmpls/ts_config_json_tmpl.txt")

	installJSPkg(do, "typescript")
	installJSPkg(do, "vite")
	installJSPkg(do, fmt.Sprintf("vorma@%s", vorma.Internal__GetCurrentNPMVersion()))
	installJSPkg(do, resolveUIVitePlugin(do))

	if do.UIVariant == "react" {
		do.tmplWriteMust("frontend/src/vorma.entry.tsx", "tmpls/frontend_entry_tsx_react_tmpl.txt")

		installJSPkg(do, "react")
		installJSPkg(do, "react-dom")
		installJSPkg(do, "@types/react")
		installJSPkg(do, "@types/react-dom")
	}

	if do.UIVariant == "solid" {
		do.tmplWriteMust("frontend/src/vorma.entry.tsx", "tmpls/frontend_entry_tsx_solid_tmpl.txt")

		installJSPkg(do, "solid-js")
	}

	if do.UIVariant == "preact" {
		do.tmplWriteMust("frontend/src/vorma.entry.tsx", "tmpls/frontend_entry_tsx_preact_tmpl.txt")

		installJSPkg(do, "preact")
		installJSPkg(do, "@preact/signals")
	}

	if do.DeploymentTarget == "vercel" {
		installJSPkg(do, "@vercel/node")
	}

	if do.IncludeTailwind {
		installJSPkg(do, "@tailwindcss/vite")
		installJSPkg(do, "tailwindcss")
		strWriteMust("frontend/src/styles/tailwind.css", "tmpls/frontend_css_tailwind_css_str.txt")
	}

	// write assets
	fileWriteMust("frontend/assets/favicon.svg", "assets/favicon.svg")

	// tidy go modules
	if err := executil.RunCmd("go", "mod", "tidy"); err != nil {
		panic("failed to tidy go modules: " + err.Error())
	}

	// build once (no binary)
	if err := executil.RunCmd("go", "run", "./backend/cmd/build", "--no-binary"); err != nil {
		panic("failed to run build command: " + err.Error())
	}

	fmt.Println()
	fmt.Println("✨ SUCCESS! Your Vorma app is ready.")
	fmt.Println()
	if o.CreatedInDir != "" {
		fmt.Printf("💻 Run `cd %s && %s dev` to start the development server.\n",
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

func (do derivedOptions) ResolveJSPackageManagerRunScriptPrefix() string {
	cmd := "npm run"
	if do.JSPackageManager != "npm" {
		cmd = do.JSPackageManager
	}
	return cmd
}

func (do derivedOptions) ResolveJSPackageManagerInstallCmd() string {
	pm := do.JSPackageManager
	switch pm {
	case "npm":
		return "npm i"
	case "pnpm":
		return "pnpm i"
	case "yarn":
		return "yarn"
	case "bun":
		return "bun i"
	}
	panic("unknown JSPackageManager: " + pm)
}

func installJSPkg(do derivedOptions, pkg string) {
	var cmd string

	switch do.JSPackageManager {
	case "npm":
		cmd = "npm i -D"
	case "pnpm":
		cmd = "pnpm add -D"
	case "yarn":
		cmd = "yarn add -D"
	case "bun":
		cmd = "bun add -d"
	}

	cmd += " " + pkg

	split := strings.Split(cmd, " ")
	err := executil.RunCmd(split...)
	if err != nil {
		panic("failed to install JS package: " + pkg + ": " + err.Error())
	}
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
