package main

import (
	"os"
	"strings"

	t "github.com/vormadev/vorma/kit/cliutil"
	"github.com/vormadev/vorma/kit/parseutil"
)

func main() {
	// Handle main package
	lines, versionLine, currentVersion := parseutil.PackageJSONFromFile("./package.json")

	// Show current tag
	t.Plain("current version: ")
	t.Green(currentVersion)
	t.NewLine()

	// Ask for new version
	t.Blue("what is the new version? ")
	version, err := t.NewReader().ReadString('\n')
	if err != nil {
		t.Exit("failed to read version", err)
	}

	trimmedVersion := strings.TrimSpace(version)
	if trimmedVersion == "" {
		t.Exit("version is empty", nil)
	}

	// Show new tag
	t.Plain("Result: ")
	t.Red(currentVersion)
	t.Plain("  -->  ")
	t.Green(trimmedVersion)
	t.NewLine()

	// Ask for confirmation
	t.Blue("is this correct? ")
	t.RequireYes("aborted")

	lines[versionLine] = strings.Replace(lines[versionLine], currentVersion, trimmedVersion, 1)

	// Ask for write confirmation
	t.Blue("write new version ")
	t.Green(trimmedVersion)
	t.Blue(" to package.json? ")
	t.RequireYes("aborted")

	// Write the new version to the file
	if err = os.WriteFile("./package.json", []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		t.Exit("failed to write file", err)
	}

	// Update create package version
	createPackagePath := "./internal/framework/_typescript/create/package.json"
	createLines, createVersionLine, createCurrentVersion := parseutil.PackageJSONFromFile(createPackagePath)

	t.Plain("Updating create package: ")
	t.Red(createCurrentVersion)
	t.Plain(" --> ")
	t.Green(trimmedVersion)
	t.NewLine()

	createLines[createVersionLine] = strings.Replace(createLines[createVersionLine], createCurrentVersion, trimmedVersion, 1)
	if err = os.WriteFile(createPackagePath, []byte(strings.Join(createLines, "\n")+"\n"), 0644); err != nil {
		t.Exit("failed to write create package.json", err)
	}

	// Sanity check
	_, _, newCurrentVersion := parseutil.PackageJSONFromFile("./package.json")
	if newCurrentVersion != trimmedVersion {
		t.Exit("failed to update version", nil)
	}

	isPre := strings.Contains(newCurrentVersion, "pre")
	if isPre {
		t.Plain("pre-release version detected")
		t.NewLine()
	}

	// Run prep
	cmd := t.Cmd("make", "tsprepforpub")
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
