package main

import (
	"os"
	"strings"

	"github.com/vormadev/vorma/internal/pkg/stringutil"
	"github.com/vormadev/vorma/kit/jsonutil"
	t "github.com/vormadev/vorma/kit/lab/cliutil"
)

const (
	base                     = "./internal/pkg/npm/"
	root_rel_base_pkg_json   = base + "package.json"
	root_rel_create_pkg_json = base + "vorma/create/package.json"
)

func main() {
	// Handle main package
	lines, version_line, current_v := pkg_json_from_file(root_rel_base_pkg_json)

	// Show current tag
	t.Plain("current version: ")
	t.Green(current_v)
	t.NewLine()

	// Ask for new version
	t.Blue("what is the new version? ")
	version, err := t.NewReader().ReadString('\n')
	if err != nil {
		t.Exit("failed to read version", err)
	}

	trimmed_v := strings.TrimSpace(version)
	if trimmed_v == "" {
		t.Exit("version is empty", nil)
	}

	// Show new tag
	t.Plain("Result: ")
	t.Red(current_v)
	t.Plain("  -->  ")
	t.Green(trimmed_v)
	t.NewLine()

	// Ask for confirmation
	t.Blue("is this correct? ")
	t.RequireYes("aborted")

	lines[version_line] = strings.Replace(lines[version_line], current_v, trimmed_v, 1)

	// Ask for write confirmation
	t.Blue("write new version ")
	t.Green(trimmed_v)
	t.Blue(" to package.json? ")
	t.RequireYes("aborted")

	// Write the new version to the file
	if err = os.WriteFile(root_rel_base_pkg_json, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		t.Exit("failed to write file", err)
	}

	// Update create package version
	create_lines, create_v_line, create_current_v := pkg_json_from_file(
		root_rel_create_pkg_json,
	)

	t.Plain("Updating create package: ")
	t.Red(create_current_v)
	t.Plain(" --> ")
	t.Green(trimmed_v)
	t.NewLine()

	create_lines[create_v_line] = strings.Replace(
		create_lines[create_v_line],
		create_current_v,
		trimmed_v,
		1,
	)
	if err = os.WriteFile(root_rel_create_pkg_json, []byte(strings.Join(create_lines, "\n")+"\n"), 0644); err != nil {
		t.Exit("failed to write create package.json", err)
	}

	// Sanity checks
	_, _, new_current_v := pkg_json_from_file(root_rel_base_pkg_json)
	if new_current_v != trimmed_v {
		t.Exit("failed to update version", nil)
	}
	_, _, new_create_current_v := pkg_json_from_file(root_rel_create_pkg_json)
	if new_create_current_v != trimmed_v {
		t.Exit("failed to update version", nil)
	}

	is_pre := strings.Contains(new_current_v, "pre")
	if is_pre {
		t.Plain("pre-release version detected")
		t.NewLine()
	}

	// Run prep
	cmd := t.Cmd("make", "prepforpub")
	t.MustRun(cmd, "prep failed")

	// Ask whether to initiate a new build?
	t.Blue("emit a new build to ./npm_dist?  ")
	t.RequireYes("aborted")

	cmd = t.Cmd("make", "npmbuild")
	t.MustRun(cmd, "npm dist build failed")

	t.NewLine()
	t.Green("npm bump complete. ready to publish.")
	t.NewLine()
}

func pkg_json_from_str(content string) ([]string, int, string) {
	lines, err := stringutil.CollectLines(content)
	if err != nil {
		panic(err)
	}
	v_line := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), `"version":`) {
			v_line = i
			break
		}
	}
	if v_line == -1 {
		panic("version line not found")
	}
	v_map, err := jsonutil.Parse[map[string]any]([]byte(content))
	if err != nil {
		panic(err)
	}
	current_v := v_map["version"].(string)
	if current_v == "" {
		panic("version not found")
	}
	return lines, v_line, current_v
}

func pkg_json_from_file(target_file string) ([]string, int, string) {
	file, err := os.ReadFile(target_file)
	if err != nil {
		panic(err)
	}
	return pkg_json_from_str(string(file))
}
