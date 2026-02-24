package vorma

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// This contract protects runtime/buildtime separation for shipped binaries:
// runtime packages must not pull build/dev tooling dependencies transitively.

var forbiddenBuildtimeDependencyPrefixes = []string{
	"github.com/vormadev/vorma/vormabuild",
	"github.com/vormadev/vorma/wave/wavebuild",
	"github.com/vormadev/vorma/wave/wavedev",
	"github.com/vormadev/vorma/lab/vitecmd",
	"github.com/evanw/esbuild/",
	"github.com/fsnotify/fsnotify",
	"github.com/gorilla/websocket",
}

func TestRuntimeDependencyContract_WaveExcludesBuildtimeToolingDeps(
	t *testing.T,
) {
	assertPackageDependencyPrefixesAbsent(
		t,
		"github.com/vormadev/vorma/wave",
		forbiddenBuildtimeDependencyPrefixes,
	)
}

func TestRuntimeDependencyContract_VormaExcludesBuildtimeToolingDeps(
	t *testing.T,
) {
	assertPackageDependencyPrefixesAbsent(
		t,
		"github.com/vormadev/vorma",
		forbiddenBuildtimeDependencyPrefixes,
	)
}

func assertPackageDependencyPrefixesAbsent(
	t *testing.T,
	importPath string,
	forbiddenPrefixes []string,
) {
	t.Helper()

	deps := listGoPackageDependencies(t, importPath)
	var found []string
	for dep := range deps {
		for _, forbiddenPrefix := range forbiddenPrefixes {
			if dep == forbiddenPrefix ||
				strings.HasPrefix(dep, forbiddenPrefix) {
				found = append(found, dep)
				break
			}
		}
	}
	if len(found) > 0 {
		slices.Sort(found)
		t.Fatalf(
			"%s transitively depends on buildtime/tooling packages: %v",
			importPath,
			found,
		)
	}
}

func listGoPackageDependencies(
	t *testing.T,
	importPath string,
) map[string]struct{} {
	t.Helper()

	cmd := exec.Command(
		"go",
		"list",
		"-deps",
		"-f",
		"{{.ImportPath}}",
		importPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf(
			"go list dependencies for %s: %v\n%s",
			importPath,
			err,
			string(output),
		)
	}

	dependencies := make(map[string]struct{})
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		dependencyImportPath := strings.TrimSpace(scanner.Text())
		if dependencyImportPath == "" {
			continue
		}
		dependencies[dependencyImportPath] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan go list output for %s: %v", importPath, err)
	}
	if len(dependencies) == 0 {
		t.Fatalf("go list returned zero dependencies for %s", importPath)
	}

	if _, ok := dependencies[importPath]; !ok {
		t.Fatalf("go list output for %s omitted package itself", importPath)
	}

	return dependencies
}

func TestRuntimeDependencyContract_DependencySetSanity(t *testing.T) {
	dependencies := listGoPackageDependencies(
		t,
		"github.com/vormadev/vorma/wave/wavedev/devserver",
	)

	var hasAnyBuildtimeSentinel bool
	for _, sentinelPrefix := range []string{
		"github.com/vormadev/vorma/lab/vitecmd",
		"github.com/evanw/esbuild/",
		"github.com/fsnotify/fsnotify",
	} {
		if hasDependencyWithPrefix(dependencies, sentinelPrefix) {
			hasAnyBuildtimeSentinel = true
			break
		}
	}

	if !hasAnyBuildtimeSentinel {
		t.Fatalf(
			"wave/wavedev/devserver dependency graph unexpectedly lacks expected buildtime sentinels: %s",
			fmt.Sprint([]string{
				"github.com/vormadev/vorma/lab/vitecmd",
				"github.com/evanw/esbuild/",
				"github.com/fsnotify/fsnotify",
			}),
		)
	}
}

func hasDependencyWithPrefix(
	dependencies map[string]struct{},
	prefix string,
) bool {
	for dependencyImportPath := range dependencies {
		if dependencyImportPath == prefix ||
			strings.HasPrefix(dependencyImportPath, prefix) {
			return true
		}
	}
	return false
}
