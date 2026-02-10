package specutil

import (
	"encoding/json"
	"fmt"
	"os"
)

type PhaseStatus struct {
	SchemaVersion  string `json:"schema_version"`
	Phase1Status   string `json:"phase1_status"`
	Phase2Status   string `json:"phase2_status"`
	Phase3Status   string `json:"phase3_status"`
	LastUpdatedUTC string `json:"last_updated_utc"`
}

func ParsePhaseStatusFile(path string) (*PhaseStatus, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	status := PhaseStatus{}
	if err := json.Unmarshal(raw, &status); err != nil {
		return nil, fmt.Errorf("invalid phase status JSON in %s: %w", path, err)
	}
	if status.SchemaVersion != "1.0.0" {
		return nil, fmt.Errorf("invalid phase status schema_version in %s: %s", path, status.SchemaVersion)
	}

	for _, value := range []struct {
		key string
		val string
	}{
		{key: "phase1_status", val: status.Phase1Status},
		{key: "phase2_status", val: status.Phase2Status},
		{key: "phase3_status", val: status.Phase3Status},
	} {
		switch value.val {
		case "INCOMPLETE", "COMPLETE", "BLOCKED":
		default:
			return nil, fmt.Errorf("invalid %s value: %s", value.key, value.val)
		}
	}

	if !IsUTCRFC3339(status.LastUpdatedUTC) {
		return nil, fmt.Errorf("invalid last_updated_utc value: %s", status.LastUpdatedUTC)
	}

	return &status, nil
}
