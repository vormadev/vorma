package specutil

import (
	"encoding/json"
	"fmt"
	"os"
)

type DecisionRow struct {
	Line              int      `json:"-"`
	DecisionID        string   `json:"decision_id"`
	Date              string   `json:"date"`
	Status            string   `json:"status"`
	PackagePaths      []string `json:"package_paths"`
	Question          string   `json:"question"`
	OptionsConsidered []string `json:"options_considered"`
	SelectedOption    string   `json:"selected_option"`
	Rationale         string   `json:"rationale"`
	EvidenceRefs      []string `json:"evidence_refs"`
}

type DecisionsDoc struct {
	SchemaVersion string        `json:"schema_version"`
	Decisions     []DecisionRow `json:"decisions"`
}

func ParseDecisionsFile(path string) ([]DecisionRow, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	doc := DecisionsDoc{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("invalid decisions JSON in %s: %w", path, err)
	}
	if doc.SchemaVersion != "1.0.0" {
		return nil, fmt.Errorf("invalid decisions schema_version in %s: %s", path, doc.SchemaVersion)
	}

	rows := make([]DecisionRow, 0, len(doc.Decisions))
	for i, row := range doc.Decisions {
		row.Line = i + 1
		rows = append(rows, row)
	}
	return rows, nil
}
