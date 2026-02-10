package specutil

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func CommandOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), strings.TrimSpace(stderr.String()))
		}
		return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return out.String(), nil
}

func CommandLines(name string, args ...string) ([]string, error) {
	out, err := CommandOutput(name, args...)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(out, "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result, nil
}

func GitLines(args ...string) ([]string, error) {
	return CommandLines("git", args...)
}

func GitOutput(args ...string) (string, error) {
	return CommandOutput("git", args...)
}

func ChangedFiles() ([]string, error) {
	diff, err := GitLines("diff", "--name-only", "--diff-filter=ACMRTD", "HEAD")
	if err != nil {
		return nil, err
	}
	untracked, err := GitLines("ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(diff)+len(untracked))
	for _, file := range append(diff, untracked...) {
		if _, ok := seen[file]; ok {
			continue
		}
		seen[file] = struct{}{}
		out = append(out, file)
	}
	sort.Strings(out)
	return out, nil
}

func IsGitFileChanged(path string) (bool, error) {
	lines, err := GitLines("diff", "--name-only", "HEAD", "--", path)
	if err != nil {
		return false, err
	}
	return len(lines) > 0, nil
}

func GitDiff(path string, unified int) (string, error) {
	return GitOutput("diff", fmt.Sprintf("--unified=%d", unified), "HEAD", "--", path)
}

func GitFileAtHEAD(path string) (string, error) {
	out, err := GitOutput("show", "HEAD:"+path)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "exists on disk, but not in 'HEAD'") ||
			strings.Contains(msg, "does not exist in 'HEAD'") {
			return "", os.ErrNotExist
		}
		return "", err
	}
	return out, nil
}

func CurrentBranch() (string, error) {
	out, err := GitOutput("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	branch := strings.TrimSpace(out)
	if branch == "" {
		return "", fmt.Errorf("current branch is empty")
	}
	return branch, nil
}

func CurrentWorktreeRoot() (string, error) {
	out, err := GitOutput("rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	root := strings.TrimSpace(out)
	if root == "" {
		return "", fmt.Errorf("current worktree root is empty")
	}
	return filepath.Clean(root), nil
}

func CurrentClaimContext() (string, error) {
	root, err := CurrentWorktreeRoot()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(root))
	return hex.EncodeToString(sum[:]), nil
}

func CurrentClaimActor() (string, error) {
	candidates := []string{
		strings.TrimSpace(os.Getenv("SPEC_AGENT_ID")),
		strings.TrimSpace(os.Getenv("USER")),
		strings.TrimSpace(os.Getenv("LOGNAME")),
	}
	if candidates[0] == "" && candidates[1] == "" && candidates[2] == "" {
		if email, err := GitOutput("config", "--get", "user.email"); err == nil {
			candidates = append(candidates, strings.TrimSpace(email))
		}
	}

	seed := ""
	for _, candidate := range candidates {
		if candidate != "" {
			seed = strings.ToLower(candidate)
			break
		}
	}
	if seed == "" {
		return "", fmt.Errorf("unable to determine claim actor identity (set SPEC_AGENT_ID or configure git user.email)")
	}

	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:]), nil
}

func GitCommonDir() (string, error) {
	out, err := GitOutput("rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	dir := strings.TrimSpace(out)
	if dir == "" {
		return "", fmt.Errorf("git common dir is empty")
	}
	if !filepath.IsAbs(dir) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(cwd, dir)
	}
	return filepath.Clean(dir), nil
}

func DispatchLockPath() (string, error) {
	commonDir, err := GitCommonDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(commonDir, "spec-dispatch.lock"), nil
}

func DispatchStatePath() (string, error) {
	commonDir, err := GitCommonDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(commonDir, "spec-dispatch-state.json"), nil
}
