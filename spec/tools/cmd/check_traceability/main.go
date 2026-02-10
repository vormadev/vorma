package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/vormadev/vorma/spec/tools/internal/specutil"
)

var (
	reGoal = regexp.MustCompile(`^GOAL-[0-9]{4}$`)
	reCap  = regexp.MustCompile(`^CAP-[A-Z0-9-]+-[0-9]{4}$`)
	reReq  = regexp.MustCompile(`^REQ-[A-Z0-9-]+-[0-9]{4}$`)
	reEvid = regexp.MustCompile(`^EVID-[A-Z0-9-]+-[0-9]{4}$`)
)

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func asString(obj map[string]any, key string) string {
	v, ok := obj[key]
	if !ok {
		fail("traceability entry missing key: %s", key)
	}
	s, ok := v.(string)
	if !ok {
		fail("traceability key %s must be string", key)
	}
	if strings.TrimSpace(s) == "" {
		fail("traceability key %s must be non-empty", key)
	}
	if strings.Contains(strings.ToUpper(s), "TODO") {
		fail("traceability key %s must not contain TODO", key)
	}
	return s
}

func asStringArray(obj map[string]any, key string, min int) []string {
	v, ok := obj[key]
	if !ok {
		fail("traceability entry missing key: %s", key)
	}
	raw, ok := v.([]any)
	if !ok {
		fail("traceability key %s must be an array", key)
	}
	if len(raw) < min {
		fail("traceability key %s must contain at least %d item(s)", key, min)
	}
	out := make([]string, 0, len(raw))
	for i, item := range raw {
		s, ok := item.(string)
		if !ok {
			fail("traceability key %s[%d] must be a string", key, i)
		}
		s = strings.TrimSpace(s)
		if s == "" {
			fail("traceability key %s[%d] must be non-empty", key, i)
		}
		if strings.Contains(strings.ToUpper(s), "TODO") {
			fail("traceability key %s[%d] must not contain TODO", key, i)
		}
		out = append(out, s)
	}
	return out
}

func extractJSONBlock(raw string) (string, error) {
	startMarker := "```json\n"
	start := strings.Index(raw, startMarker)
	if start == -1 {
		return "", fmt.Errorf("TRACEABILITY.md is missing a ```json fenced block")
	}
	end := strings.Index(raw[start+len(startMarker):], "\n```")
	if end == -1 {
		return "", fmt.Errorf("TRACEABILITY.md JSON fenced block terminator is missing")
	}
	end += start + len(startMarker)
	return strings.TrimSpace(raw[start+len(startMarker) : end]), nil
}

func main() {
	raw, err := os.ReadFile("spec/TRACEABILITY.md")
	if err != nil {
		fail("%v", err)
	}

	block, err := extractJSONBlock(string(raw))
	if err != nil {
		fail("%v", err)
	}

	dec := json.NewDecoder(strings.NewReader(block))
	dec.UseNumber()

	var entries []map[string]any
	if err := dec.Decode(&entries); err != nil {
		fail("TRACEABILITY.md JSON is invalid: %v", err)
	}

	goalIDs := map[string]struct{}{}
	for i, entry := range entries {
		required := []string{
			"user_goal_id",
			"user_goal_summary",
			"capability_ids",
			"requirement_ids",
			"owner_package_paths",
			"upstream_requirement_refs",
			"evidence_ids",
		}
		for _, key := range required {
			if _, ok := entry[key]; !ok {
				fail("traceability entry %d missing key: %s", i, key)
			}
		}
		for key := range entry {
			found := false
			for _, requiredKey := range required {
				if key == requiredKey {
					found = true
					break
				}
			}
			if !found {
				fail("traceability entry %d has unknown key: %s", i, key)
			}
		}

		goalID := asString(entry, "user_goal_id")
		if !reGoal.MatchString(goalID) {
			fail("traceability entry %d has invalid user_goal_id: %s", i, goalID)
		}
		if _, exists := goalIDs[goalID]; exists {
			fail("traceability contains duplicate user_goal_id: %s", goalID)
		}
		goalIDs[goalID] = struct{}{}
		_ = asString(entry, "user_goal_summary")

		caps := asStringArray(entry, "capability_ids", 1)
		for _, capID := range caps {
			if !reCap.MatchString(capID) {
				fail("traceability entry %d has invalid capability_id: %s", i, capID)
			}
		}

		reqs := asStringArray(entry, "requirement_ids", 1)
		for _, reqID := range reqs {
			if !reReq.MatchString(reqID) {
				fail("traceability entry %d has invalid requirement_id: %s", i, reqID)
			}
		}

		ownerPaths := asStringArray(entry, "owner_package_paths", 1)
		for _, p := range ownerPaths {
			if !strings.HasPrefix(p, "spec/packages/") {
				fail("traceability entry %d owner_package_path must start with spec/packages/: %s", i, p)
			}
			if !specutil.IsRelativeRepoPath(p) {
				fail("traceability entry %d owner_package_path must be repository-relative: %s", i, p)
			}
		}

		upstreamRefs := asStringArray(entry, "upstream_requirement_refs", 0)
		for _, ref := range upstreamRefs {
			if !reReq.MatchString(ref) {
				fail("traceability entry %d has invalid upstream_requirement_ref: %s", i, ref)
			}
		}

		evidenceIDs := asStringArray(entry, "evidence_ids", 1)
		for _, evidID := range evidenceIDs {
			if !reEvid.MatchString(evidID) {
				fail("traceability entry %d has invalid evidence_id: %s", i, evidID)
			}
		}
	}
}
