package specutil

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

type PhaseStatus struct {
	Phase1Status   string
	Phase2Status   string
	Phase3Status   string
	LastUpdatedUTC string
}

var phaseEntryRe = regexp.MustCompile(`\b(phase1_status|phase2_status|phase3_status|last_updated_utc):\s*([A-Z0-9:-]+)`)

func ParsePhaseStatusFile(path string) (*PhaseStatus, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	values := map[string]string{}
	matches := phaseEntryRe.FindAllStringSubmatch(string(raw), -1)
	for _, m := range matches {
		key := m[1]
		value := m[2]
		if _, exists := values[key]; exists {
			return nil, fmt.Errorf("duplicate phase status key: %s", key)
		}
		values[key] = value
	}

	required := []string{"phase1_status", "phase2_status", "phase3_status", "last_updated_utc"}
	for _, key := range required {
		if strings.TrimSpace(values[key]) == "" {
			return nil, fmt.Errorf("missing required phase status key: %s", key)
		}
	}

	for _, key := range []string{"phase1_status", "phase2_status", "phase3_status"} {
		switch values[key] {
		case "INCOMPLETE", "COMPLETE", "BLOCKED":
		default:
			return nil, fmt.Errorf("invalid %s value: %s", key, values[key])
		}
	}

	if !IsUTCRFC3339(values["last_updated_utc"]) {
		return nil, fmt.Errorf("invalid last_updated_utc value: %s", values["last_updated_utc"])
	}

	return &PhaseStatus{
		Phase1Status:   values["phase1_status"],
		Phase2Status:   values["phase2_status"],
		Phase3Status:   values["phase3_status"],
		LastUpdatedUTC: values["last_updated_utc"],
	}, nil
}
