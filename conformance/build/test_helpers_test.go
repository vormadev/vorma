package build_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

type buildFixture struct {
	root       string
	configPath string
	distDir    string
	tsGenDir   string
}

type buildFixtureOptions struct {
	includeDefaults *bool
	viteCmdPath     string
	routeDefs       string
}

func newBuildFixture(t *testing.T, opts *buildFixtureOptions) *buildFixture {
	t.Helper()

	if opts == nil {
		opts = &buildFixtureOptions{}
	}

	root := t.TempDir()
	distDir := filepath.Join(root, "dist")
	privateDir := filepath.Join(root, "assets", "private")
	publicDir := filepath.Join(root, "assets", "public")
	tsGenDir := filepath.Join(root, "frontend", "src", "vorma.gen")

	mustMkdirAll(t, filepath.Join(root, "frontend", "src", "routes"))
	mustMkdirAll(t, privateDir)
	mustMkdirAll(t, publicDir)
	mustMkdirAll(t, tsGenDir)

	mustWriteFile(t, filepath.Join(privateDir, "index.html"), `<!doctype html><html><head>{{.VormaHeadEls}}</head><body><div id="{{.VormaRootID}}"></div>{{.VormaBodyScripts}}{{.VormaSSRScript}}</body></html>`)
	mustWriteFile(t, filepath.Join(publicDir, "logo.svg"), `<svg xmlns="http://www.w3.org/2000/svg"></svg>`)
	mustWriteFile(t, filepath.Join(root, "frontend", "src", "vorma.entry.tsx"), `export default function App(){ return null }`)

	defaultRouteDefs := `import { route } from "vorma/client";
route("", "./routes/root.tsx");
route("/home", "./routes/home.tsx");
route("/default-key", "./routes/default_key.tsx");
`
	routeDefs := defaultRouteDefs
	if opts.routeDefs != "" {
		routeDefs = opts.routeDefs
	}
	mustWriteFile(t, filepath.Join(root, "frontend", "src", "vorma.routes.ts"), routeDefs)

	mustWriteFile(t, filepath.Join(root, "frontend", "src", "routes", "root.tsx"), `export default function Root(){ return null }`)
	mustWriteFile(t, filepath.Join(root, "frontend", "src", "routes", "home.tsx"), `export default function Home(){ return null }`)
	mustWriteFile(t, filepath.Join(root, "frontend", "src", "routes", "default_key.tsx"), `export default function DefaultKey(){ return null }`)

	vormaCfg := map[string]any{
		"MainBuildEntry":       "cmd/build/main.go",
		"UIVariant":            "react",
		"HTMLTemplateLocation": "index.html",
		"ClientEntry":          "frontend/src/vorma.entry.tsx",
		"ClientRouteDefsFile":  "frontend/src/vorma.routes.ts",
		"TSGenOutDir":          "frontend/src/vorma.gen",
	}
	if opts.includeDefaults != nil {
		vormaCfg["IncludeDefaults"] = *opts.includeDefaults
	}

	cfg := map[string]any{
		"Core": map[string]any{
			"ConfigLocation": "wave.config.json",
			"MainAppEntry":   "cmd/serve/main.go",
			"DistDir":        "dist",
			"StaticAssetDirs": map[string]any{
				"Private": "assets/private",
				"Public":  "assets/public",
			},
			"PublicPathPrefix": "/",
		},
		"Vorma": vormaCfg,
	}
	if opts.viteCmdPath != "" {
		cfg["Vite"] = map[string]any{
			"JSPackageManagerBaseCmd": opts.viteCmdPath,
		}
	}

	configPath := filepath.Join(root, "wave.config.json")
	mustWriteJSONFile(t, configPath, cfg)

	return &buildFixture{
		root:       root,
		configPath: configPath,
		distDir:    distDir,
		tsGenDir:   tsGenDir,
	}
}

func writeFakeViteCmd(t *testing.T, root string) string {
	t.Helper()

	cmdPath := filepath.Join(root, "fakepm.sh")
	script := `#!/bin/sh
set -eu
outdir=""
manifest=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --outDir)
      outdir="$2"
      shift 2
      ;;
    --manifest)
      manifest="$2"
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done
if [ -z "$outdir" ] || [ -z "$manifest" ]; then
  echo "missing --outDir or --manifest" >&2
  exit 2
fi
mkdir -p "$outdir"
cat > "$outdir/$manifest" <<'JSON'
{
  "frontend/src/vorma.entry.tsx": {
    "src": "frontend/src/vorma.entry.tsx",
    "file": "vorma_out_vite_entry.js",
    "isEntry": true,
    "imports": ["frontend/src/routes/root.tsx"],
    "css": ["vorma_out_vite_entry.css"]
  },
  "frontend/src/routes/root.tsx": {
    "src": "frontend/src/routes/root.tsx",
    "file": "vorma_out_vite_root.js",
    "isDynamicEntry": true,
    "imports": []
  },
  "frontend/src/routes/home.tsx": {
    "src": "frontend/src/routes/home.tsx",
    "file": "vorma_out_vite_home.js",
    "isDynamicEntry": true,
    "imports": []
  },
  "frontend/src/routes/default_key.tsx": {
    "src": "frontend/src/routes/default_key.tsx",
    "file": "vorma_out_vite_default.js",
    "isDynamicEntry": true,
    "imports": []
  }
}
JSON
touch "$outdir/vorma_out_vite_entry.js"
touch "$outdir/vorma_out_vite_root.js"
touch "$outdir/vorma_out_vite_home.js"
touch "$outdir/vorma_out_vite_default.js"
touch "$outdir/vorma_out_vite_entry.css"
`
	mustWriteFile(t, cmdPath, script)
	if err := os.Chmod(cmdPath, 0o755); err != nil {
		t.Fatalf("chmod fake vite cmd: %v", err)
	}
	return cmdPath
}

func runBuildProbe(t *testing.T, fixture *buildFixture, args ...string) (string, error) {
	t.Helper()
	return runBuildProbeWithEnv(t, fixture, nil, args...)
}

func runBuildProbeWithEnv(t *testing.T, fixture *buildFixture, extraEnv map[string]string, args ...string) (string, error) {
	t.Helper()

	goRunArgs := []string{
		"run",
		filepath.Join(repoRoot(t), "conformance", "build", "cmd", "buildprobe"),
	}
	goRunArgs = append(goRunArgs, args...)

	cmd := exec.Command("go", goRunArgs...)
	cmd.Dir = repoRoot(t)
	env := append(
		os.Environ(),
		"BUILDPROBE_WORKDIR="+fixture.root,
		"BUILDPROBE_CONFIG="+fixture.configPath,
		"BUILDPROBE_STATIC_ROOT=.",
		"GOCACHE=/tmp/go-build",
	)
	for k, v := range extraEnv {
		env = append(env, k+"="+v)
	}
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func runBuildProbeWithTimeout(
	t *testing.T,
	fixture *buildFixture,
	timeout time.Duration,
	extraEnv map[string]string,
	args ...string,
) (output string, err error, timedOut bool) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	goRunArgs := []string{
		"run",
		filepath.Join(repoRoot(t), "conformance", "build", "cmd", "buildprobe"),
	}
	goRunArgs = append(goRunArgs, args...)

	cmd := exec.Command("go", goRunArgs...)
	cmd.Dir = repoRoot(t)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	env := append(
		os.Environ(),
		"BUILDPROBE_WORKDIR="+fixture.root,
		"BUILDPROBE_CONFIG="+fixture.configPath,
		"BUILDPROBE_STATIC_ROOT=.",
		"GOCACHE=/tmp/go-build",
	)
	for k, v := range extraEnv {
		env = append(env, k+"="+v)
	}
	cmd.Env = env

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if startErr := cmd.Start(); startErr != nil {
		return out.String(), startErr, false
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case runErr := <-done:
		return out.String(), runErr, false
	case <-ctx.Done():
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		<-done
		return out.String(), ctx.Err(), true
	}
}

func addFixtureCLIGoCommands(t *testing.T, fixture *buildFixture) {
	t.Helper()

	mustWriteFile(t, filepath.Join(fixture.root, "go.mod"), "module fixturecli\n\ngo 1.23.0\n")
	mustWriteFile(
		t,
		filepath.Join(fixture.root, "cmd", "serve", "main.go"),
		`package main

func main() {}
`,
	)
	mustWriteFile(
		t,
		filepath.Join(fixture.root, "cmd", "build", "main.go"),
		`package main

import (
	"os"
	"strings"
)

func main() {
	marker := os.Getenv("CLI_HOOK_MARKER")
	if marker == "" {
		return
	}
	f, err := os.OpenFile(marker, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(strings.Join(os.Args[1:], " ") + "\n")
}
`,
	)
}

func mutateFixtureConfig(t *testing.T, fixture *buildFixture, mutate func(map[string]any)) {
	t.Helper()

	data, err := os.ReadFile(fixture.configPath)
	if err != nil {
		t.Fatalf("read fixture config: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal fixture config: %v", err)
	}
	mutate(cfg)
	mustWriteJSONFile(t, fixture.configPath, cfg)
}

func mustReadJSONFileMap(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read json file %s: %v", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal json file %s: %v body=%q", path, err, string(data))
	}
	return m
}

func mustAnyMap(t *testing.T, raw any, field string) map[string]any {
	t.Helper()
	m, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("expected %s as map[string]any, got %#v", field, raw)
	}
	return m
}

func findSingleFileWithPrefix(t *testing.T, dir string, prefix string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	var matches []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, prefix) {
			matches = append(matches, filepath.Join(dir, name))
		}
	}
	if len(matches) != 1 {
		t.Fatalf("expected exactly one file with prefix %q in %s, got %d (%v)", prefix, dir, len(matches), matches)
	}
	return matches[0]
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("repo root detection failed from %s: %v", wd, err)
	}
	return root
}

func mustMkdirAll(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir parent for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func mustWriteJSONFile(t *testing.T, path string, payload any) {
	t.Helper()
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal json for %s: %v", path, err)
	}
	mustWriteFile(t, path, string(b))
}

func mustStatModTime(t *testing.T, path string) time.Time {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return fi.ModTime()
}

func failf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}
