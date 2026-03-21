package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/vormadev/vorma/internal/coalescepath"
	"github.com/vormadev/vorma/internal/pkg/parseutil"
	"github.com/vormadev/vorma/kit/envutil"
	t "github.com/vormadev/vorma/lab/cliutil"
	"github.com/vormadev/vorma/lab/coalescecmd"
	"golang.org/x/term"
)

const (
	canonicalVersionFilePath = "./internal/__LAST_RELEASE.txt"
	rootPackageJSONPath      = "./package.json"
	createPackageJSONPath    = "./typescript/vorma/create/package.json"
	releaseUnsafeModeEnvVar  = "UNSAFE"
)

type packageJSONVersionFile struct {
	path           string
	lines          []string
	versionLine    int
	currentVersion string
}

type packageJSONVersion struct {
	Version string `json:"version"`
}

type npmPublishTarget struct {
	packageName string
	workingDir  string
}

type releaseProcessError struct {
	message string
	err     error
}

func main() {
	if len(os.Args) != 1 {
		t.Exit("release command does not take arguments", nil)
	}

	coalesceError := coalescecmd.Run(coalescecmd.Options{
		Key:                    coalescepath.ReleaseCommandKey,
		FailIfRunning:          []string{coalescepath.ReleaseCommandKey},
		StateRootDirectoryPath: coalescepath.StateRootDirectoryPath,
		Func: func() error {
			releaseErr := runUnifiedReleaseProcess()
			if releaseErr == nil {
				return nil
			}
			if releaseErr.err != nil {
				return fmt.Errorf("%s: %w", releaseErr.message, releaseErr.err)
			}
			return errors.New(releaseErr.message)
		},
	})
	if coalesceError != nil {
		if errors.Is(coalesceError, coalescecmd.ErrAlreadyRunning) {
			t.Exit("another release process is already running", nil)
		}
		t.Exit("release failed", coalesceError)
	}
}

func runUnifiedReleaseProcess() (releaseErr *releaseProcessError) {
	currentCanonicalVersion := readCanonicalVersionOrExit(
		canonicalVersionFilePath,
	)
	rootPackageJSON := loadPackageJSONVersionFile(rootPackageJSONPath)
	createPackageJSON := loadPackageJSONVersionFile(createPackageJSONPath)
	rootPackageJSONWasUpdated := false
	createPackageJSONWasUpdated := false
	canonicalVersionWasUpdated := false

	defer func() {
		if releaseErr == nil {
			return
		}
		rollbackCanonicalVersionIfUpdated(
			canonicalVersionWasUpdated,
			currentCanonicalVersion,
		)
		rollbackPackageJSONVersionsIfUpdated(
			rootPackageJSONWasUpdated,
			createPackageJSONWasUpdated,
			rootPackageJSON,
			createPackageJSON,
		)
	}()

	t.Plain("current version: ")
	t.Green(currentCanonicalVersion)
	t.NewLine()

	targetVersion := promptTargetVersion()
	validatePrereleaseIdentifierPolicyOrExit(targetVersion)
	releaseHasPreTag := versionHasPreReleaseTag(targetVersion)

	isUnsafeMode := envutil.GetBool(releaseUnsafeModeEnvVar, false)
	if isUnsafeMode && !releaseHasPreTag {
		t.Exit("you can't run unsafe mode on a non-pre release", nil)
	}

	t.Plain("Result: ")
	t.Red(currentCanonicalVersion)
	t.Plain("  -->  ")
	t.Green(targetVersion)
	t.NewLine()

	t.Blue("is this correct? ")
	confirmationErr := requireYesOrFail("aborted")
	if confirmationErr != nil {
		return confirmationErr
	}

	if releaseHasPreTag {
		t.Plain("pre-release version detected")
		t.NewLine()
	}

	versionFilesNeedUpdate := rootPackageJSON.currentVersion != targetVersion ||
		createPackageJSON.currentVersion != targetVersion

	if versionFilesNeedUpdate {
		t.Blue("write new version ")
		t.Green(targetVersion)
		t.Blue(" to package.json? ")
		confirmationErr = requireYesOrFail("aborted")
		if confirmationErr != nil {
			return confirmationErr
		}

		writeRootErr := writePackageJSONVersionFile(
			rootPackageJSON,
			targetVersion,
		)
		if writeRootErr != nil {
			return writeRootErr
		}
		rootPackageJSONWasUpdated = true
		t.Plain("Updating create package: ")
		t.Red(createPackageJSON.currentVersion)
		t.Plain(" --> ")
		t.Green(targetVersion)
		t.NewLine()
		writeCreateErr := writePackageJSONVersionFile(
			createPackageJSON,
			targetVersion,
		)
		if writeCreateErr != nil {
			return writeCreateErr
		}
		createPackageJSONWasUpdated = true
	}

	ensurePackageJSONErr := ensurePackageJSONVersionsMatch(targetVersion)
	if ensurePackageJSONErr != nil {
		return ensurePackageJSONErr
	}

	t.Blue("publish to npm? ")
	confirmationErr = requireYesOrFail("aborted")
	if confirmationErr != nil {
		return confirmationErr
	}

	publishVormaErr := publishNPMPackageVersionIdempotently(
		npmPublishTarget{
			packageName: "vorma",
			workingDir:  ".",
		},
		targetVersion,
		releaseHasPreTag,
	)
	if publishVormaErr != nil {
		return publishVormaErr
	}
	publishCreateErr := publishNPMPackageVersionIdempotently(
		npmPublishTarget{
			packageName: "create-vorma",
			workingDir:  "./typescript/vorma/create",
		},
		targetVersion,
		releaseHasPreTag,
	)
	if publishCreateErr != nil {
		return publishCreateErr
	}

	t.Blue("apply tag ")
	t.Green("v" + targetVersion)
	t.Blue(" and push to git? ")
	confirmationErr = requireYesOrFail("aborted")
	if confirmationErr != nil {
		return confirmationErr
	}

	if currentCanonicalVersion != targetVersion {
		writeCanonicalErr := writeCanonicalVersionFile(
			canonicalVersionFilePath,
			targetVersion,
		)
		if writeCanonicalErr != nil {
			return writeCanonicalErr
		}
		canonicalVersionWasUpdated = true
	}

	commitErr := commitAndPushReleaseChangesIdempotently(targetVersion)
	if commitErr != nil {
		return commitErr
	}

	tagErr := tagAndPublishGoProxyIdempotently(targetVersion)
	if tagErr != nil {
		return tagErr
	}

	ensureCanonicalVersionErr := ensureCanonicalVersionMatchesExpected(
		targetVersion,
	)
	if ensureCanonicalVersionErr != nil {
		return ensureCanonicalVersionErr
	}

	t.Green("release complete")
	t.NewLine()

	return nil
}

func promptTargetVersion() string {
	t.Blue("what is the new version? ")

	input, err := t.NewReader().ReadString('\n')
	if err != nil {
		t.Exit("failed to read version", err)
	}

	trimmedInput := strings.TrimSpace(input)
	if trimmedInput == "" {
		t.Exit("version is empty", nil)
	}

	normalized := strings.TrimPrefix(trimmedInput, "v")
	if normalized == "" {
		t.Exit("version is empty", nil)
	}
	return normalized
}

func validatePrereleaseIdentifierPolicyOrExit(version string) {
	prereleaseIdentifier := getPrereleaseIdentifier(version)
	if prereleaseIdentifier != "" && prereleaseIdentifier != "pre" {
		t.Exit("only prerelease tag 'pre' is allowed", nil)
	}
}

func versionHasPreReleaseTag(version string) bool {
	return getPrereleaseIdentifier(version) == "pre"
}

func getPrereleaseIdentifier(version string) string {
	prereleaseSeparatorIndex := strings.Index(version, "-")
	if prereleaseSeparatorIndex == -1 {
		return ""
	}

	buildMetadataSeparatorIndex := strings.Index(version, "+")
	prereleaseEndIndex := len(version)
	if buildMetadataSeparatorIndex != -1 &&
		buildMetadataSeparatorIndex > prereleaseSeparatorIndex {
		prereleaseEndIndex = buildMetadataSeparatorIndex
	}

	prereleaseSegment := version[prereleaseSeparatorIndex+1 : prereleaseEndIndex]
	if prereleaseSegment == "" {
		t.Exit("release version prerelease segment is empty", nil)
		return ""
	}

	if before, _, ok := strings.Cut(prereleaseSegment, "."); ok {
		return before
	}
	return prereleaseSegment
}

func publishNPMPackageVersionIdempotently(
	target npmPublishTarget,
	version string,
	hasPreReleaseTag bool,
) *releaseProcessError {
	alreadyPublished := isNPMPackageVersionPublished(
		target.packageName,
		version,
	)
	if alreadyPublished {
		t.Plain("npm package already published: ")
		t.Green(target.packageName + "@" + version)
		t.NewLine()
		ensureNPMPreTagErr := ensureNPMPreTagSynchronization(
			target.packageName,
			version,
			hasPreReleaseTag,
		)
		if ensureNPMPreTagErr != nil {
			return ensureNPMPreTagErr
		}
		return nil
	}

	publishArgs := []string{"publish", "--access", "public"}
	if hasPreReleaseTag {
		publishArgs = append(publishArgs, "--tag", "pre")
	}

	cmd := t.Cmd("npm", publishArgs...)
	cmd.Dir = target.workingDir
	publishErr := runCommand(cmd, "npm publish failed for "+target.packageName)
	if publishErr != nil {
		return publishErr
	}

	ensureNPMPreTagErr := ensureNPMPreTagSynchronization(
		target.packageName,
		version,
		hasPreReleaseTag,
	)
	if ensureNPMPreTagErr != nil {
		return ensureNPMPreTagErr
	}

	return nil
}

func isNPMPackageVersionPublished(packageName string, version string) bool {
	spec := packageName + "@" + version
	cmd := t.Cmd("npm", "view", spec, "version")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	publishedVersion := strings.Trim(strings.TrimSpace(string(output)), "\"")
	return publishedVersion == version
}

func ensureNPMPreTagSynchronization(
	packageName string,
	version string,
	hasPreReleaseTag bool,
) *releaseProcessError {
	packageVersionSpec := packageName + "@" + version
	preTagVersion, preTagExists := readNPMDistTagVersion(
		packageName,
		"pre",
	)

	if hasPreReleaseTag {
		if !preTagExists || preTagVersion != version {
			addDistTagErr := runCommand(
				t.Cmd("npm", "dist-tag", "add", packageVersionSpec, "pre"),
				"failed to synchronize npm dist-tag 'pre' for "+packageName,
			)
			if addDistTagErr != nil {
				return addDistTagErr
			}
		}
		return nil
	}

	if preTagExists && preTagVersion == version {
		removeDistTagErr := runCommand(
			t.Cmd("npm", "dist-tag", "rm", packageName, "pre"),
			"failed to remove conflicting npm dist-tag 'pre' for "+packageName,
		)
		if removeDistTagErr != nil {
			return removeDistTagErr
		}
	}

	return nil
}

func readNPMDistTagVersion(packageName string, tagName string) (string, bool) {
	cmd := t.Cmd("npm", "view", packageName, "dist-tags."+tagName)
	output, err := cmd.Output()
	if err != nil {
		return "", false
	}

	tagVersion := strings.Trim(strings.TrimSpace(string(output)), "\"")
	if tagVersion == "" {
		return "", false
	}
	return tagVersion, true
}

func commitAndPushReleaseChangesIdempotently(
	version string,
) *releaseProcessError {
	gitAddErr := runCommand(
		t.Cmd("git", "add", "."),
		"failed to stage git changes",
	)
	if gitAddErr != nil {
		return gitAddErr
	}

	hasStagedChanges, stagedChangesErr := gitHasStagedChangesOrFail()
	if stagedChangesErr != nil {
		return stagedChangesErr
	}
	if hasStagedChanges {
		commitMessage := "v" + version
		commitErr := runCommand(
			t.Cmd("git", "commit", "--no-verify", "-m", commitMessage),
			"git commit failed",
		)
		if commitErr != nil {
			return commitErr
		}
	} else {
		t.Plain("no git changes to commit")
		t.NewLine()
	}

	gitPushErr := runCommand(t.Cmd("git", "push"), "git push failed")
	if gitPushErr != nil {
		return gitPushErr
	}

	return nil
}

func gitHasStagedChangesOrFail() (bool, *releaseProcessError) {
	cmd := t.Cmd("git", "diff", "--cached", "--name-only")
	output, err := cmd.Output()
	if err != nil {
		return false, &releaseProcessError{
			message: "failed to inspect staged git changes",
			err:     err,
		}
	}
	return strings.TrimSpace(string(output)) != "", nil
}

func tagAndPublishGoProxyIdempotently(version string) *releaseProcessError {
	tagName := "v" + version

	tagExistsLocally, localTagLookupErr := gitTagExistsLocallyOrFail(tagName)
	if localTagLookupErr != nil {
		return localTagLookupErr
	}
	if !tagExistsLocally {
		createTagErr := runCommand(
			t.Cmd("git", "tag", tagName),
			"git tag creation failed",
		)
		if createTagErr != nil {
			return createTagErr
		}
	}

	tagExistsOnOrigin, originTagLookupErr := gitTagExistsOnOriginOrFail(tagName)
	if originTagLookupErr != nil {
		return originTagLookupErr
	}
	if !tagExistsOnOrigin {
		pushTagErr := runCommand(
			t.Cmd("git", "push", "origin", "refs/tags/"+tagName),
			"git tag push failed",
		)
		if pushTagErr != nil {
			return pushTagErr
		}
	}

	goListCmd := t.Cmd("go", "list", "-m", "all")
	goListCmd.Env = append(os.Environ(), "GOPROXY=proxy.golang.org")
	goListErr := runCommand(goListCmd, "go proxy update failed")
	if goListErr != nil {
		return goListErr
	}

	return nil
}

func gitTagExistsLocallyOrFail(tagName string) (bool, *releaseProcessError) {
	cmd := t.Cmd("git", "tag", "-l", tagName)
	output, err := cmd.Output()
	if err != nil {
		return false, &releaseProcessError{
			message: "failed to inspect local git tags",
			err:     err,
		}
	}
	return strings.TrimSpace(string(output)) != "", nil
}

func gitTagExistsOnOriginOrFail(tagName string) (bool, *releaseProcessError) {
	cmd := t.Cmd("git", "ls-remote", "--tags", "origin", "refs/tags/"+tagName)
	output, err := cmd.Output()
	if err != nil {
		return false, &releaseProcessError{
			message: "failed to inspect remote git tags",
			err:     err,
		}
	}
	return strings.TrimSpace(string(output)) != "", nil
}

func ensurePackageJSONVersionsMatch(targetVersion string) *releaseProcessError {
	rootPackageVersion, readRootErr := readPackageJSONVersionFromFile(
		rootPackageJSONPath,
	)
	if readRootErr != nil {
		return readRootErr
	}
	if rootPackageVersion != targetVersion {
		return &releaseProcessError{
			message: "root package.json version does not match expected target version",
			err:     nil,
		}
	}

	createPackageVersion, readCreateErr := readPackageJSONVersionFromFile(
		createPackageJSONPath,
	)
	if readCreateErr != nil {
		return readCreateErr
	}
	if createPackageVersion != targetVersion {
		return &releaseProcessError{
			message: "create package.json version does not match expected target version",
			err:     nil,
		}
	}

	return nil
}

func ensureCanonicalVersionMatchesExpected(
	targetVersion string,
) *releaseProcessError {
	canonicalVersion, readCanonicalErr := readCanonicalVersionFromFile(
		canonicalVersionFilePath,
	)
	if readCanonicalErr != nil {
		return readCanonicalErr
	}
	if canonicalVersion != targetVersion {
		return &releaseProcessError{
			message: "internal/__LAST_RELEASE.txt does not match expected target version",
			err:     nil,
		}
	}

	return nil
}

func writeCanonicalVersionFile(
	path string,
	version string,
) *releaseProcessError {
	err := os.WriteFile(path, []byte(version+"\n"), 0644)
	if err != nil {
		return &releaseProcessError{
			message: "failed to write canonical version file",
			err:     err,
		}
	}
	return nil
}

func loadPackageJSONVersionFile(path string) packageJSONVersionFile {
	lines, versionLine, currentVersion := parseutil.MustPackageJSONFromFile(
		path,
	)
	return packageJSONVersionFile{
		path:           path,
		lines:          lines,
		versionLine:    versionLine,
		currentVersion: currentVersion,
	}
}

func writePackageJSONVersionFile(
	packageJSON packageJSONVersionFile,
	newVersion string,
) *releaseProcessError {
	updatedLines := append([]string(nil), packageJSON.lines...)
	updatedLines[packageJSON.versionLine] = strings.Replace(
		updatedLines[packageJSON.versionLine],
		packageJSON.currentVersion,
		newVersion,
		1,
	)
	err := os.WriteFile(
		packageJSON.path,
		[]byte(strings.Join(updatedLines, "\n")+"\n"),
		0644,
	)
	if err != nil {
		return &releaseProcessError{
			message: "failed to write package.json version for " + packageJSON.path,
			err:     err,
		}
	}

	return nil
}

func readCanonicalVersionOrExit(path string) string {
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Exit("failed to read internal/__LAST_RELEASE.txt", err)
	}

	version := strings.TrimSpace(string(contents))
	if version == "" {
		t.Exit("internal/__LAST_RELEASE.txt is empty", nil)
	}

	return version
}

func readCanonicalVersionFromFile(path string) (string, *releaseProcessError) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", &releaseProcessError{
			message: "failed to read internal/__LAST_RELEASE.txt",
			err:     err,
		}
	}

	version := strings.TrimSpace(string(contents))
	if version == "" {
		return "", &releaseProcessError{
			message: "internal/__LAST_RELEASE.txt is empty",
			err:     nil,
		}
	}

	return version, nil
}

func readPackageJSONVersionFromFile(
	path string,
) (string, *releaseProcessError) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", &releaseProcessError{
			message: "failed to read " + path,
			err:     err,
		}
	}

	var parsedPackageJSON packageJSONVersion
	if err := json.Unmarshal(contents, &parsedPackageJSON); err != nil {
		return "", &releaseProcessError{
			message: "failed to parse " + path,
			err:     err,
		}
	}

	version := strings.TrimSpace(parsedPackageJSON.Version)
	if version == "" {
		return "", &releaseProcessError{
			message: path + " version is empty",
			err:     nil,
		}
	}
	return version, nil
}

func rollbackCanonicalVersionIfUpdated(
	canonicalVersionWasUpdated bool,
	previousCanonicalVersion string,
) {
	if !canonicalVersionWasUpdated {
		return
	}

	rollbackErr := writeCanonicalVersionFile(
		canonicalVersionFilePath,
		previousCanonicalVersion,
	)
	if rollbackErr != nil {
		t.Plain("WARNING: failed to rollback internal/__LAST_RELEASE.txt: ")
		t.Red(rollbackErr.err.Error())
		t.NewLine()
		return
	}

	t.Plain("rolled back internal/__LAST_RELEASE.txt after failure")
	t.NewLine()
}

func rollbackPackageJSONVersionsIfUpdated(
	rootPackageJSONWasUpdated bool,
	createPackageJSONWasUpdated bool,
	rootPackageJSON packageJSONVersionFile,
	createPackageJSON packageJSONVersionFile,
) {
	if rootPackageJSONWasUpdated {
		rollbackRootErr := writePackageJSONVersionFile(
			rootPackageJSON,
			rootPackageJSON.currentVersion,
		)
		if rollbackRootErr != nil {
			t.Plain("WARNING: failed to rollback package.json: ")
			if rollbackRootErr.err != nil {
				t.Red(rollbackRootErr.err.Error())
			} else {
				t.Red(rollbackRootErr.message)
			}
			t.NewLine()
		} else {
			t.Plain("rolled back package.json after failure")
			t.NewLine()
		}
	}

	if createPackageJSONWasUpdated {
		rollbackCreateErr := writePackageJSONVersionFile(
			createPackageJSON,
			createPackageJSON.currentVersion,
		)
		if rollbackCreateErr != nil {
			t.Plain(
				"WARNING: failed to rollback typescript/vorma/create/package.json: ",
			)
			if rollbackCreateErr.err != nil {
				t.Red(rollbackCreateErr.err.Error())
			} else {
				t.Red(rollbackCreateErr.message)
			}
			t.NewLine()
		} else {
			t.Plain("rolled back typescript/vorma/create/package.json after failure")
			t.NewLine()
		}
	}
}

func runCommand(cmd *exec.Cmd, failMsg string) *releaseProcessError {
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return &releaseProcessError{
			message: failMsg,
			err:     err,
		}
	}
	return nil
}

func requireYesOrFail(failMsg string) *releaseProcessError {
	t.Plain("(y/n) ")

	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return &releaseProcessError{
			message: "failed to set terminal raw mode",
			err:     err,
		}
	}

	buf := make([]byte, 1)
	_, err = os.Stdin.Read(buf)

	term.Restore(fd, oldState)

	if err != nil {
		return &releaseProcessError{
			message: "failed to read input",
			err:     err,
		}
	}

	fmt.Printf("%c", buf[0])
	t.NewLine()

	if buf[0] != 'y' && buf[0] != 'Y' {
		return &releaseProcessError{
			message: failMsg,
			err:     nil,
		}
	}
	return nil
}
