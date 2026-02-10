package specutil

import (
	"fmt"
	"os"
	"strings"
)

type DecisionRow struct {
	Line              int
	DecisionID        string
	Date              string
	Status            string
	PackagePath       string
	Question          string
	OptionsConsidered string
	SelectedOption    string
	Rationale         string
	Evidence          string
}

var decisionsHeader = []string{
	"Decision ID",
	"Date",
	"Status",
	"Package Path",
	"Question",
	"Options Considered",
	"Selected Option",
	"Rationale",
	"Evidence",
}

func parseMarkdownRow(line string) ([]string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "|") || !strings.HasSuffix(trimmed, "|") {
		return nil, false
	}
	parts := strings.Split(trimmed, "|")
	if len(parts) < 3 {
		return nil, false
	}
	out := make([]string, 0, len(parts)-2)
	for i := 1; i < len(parts)-1; i++ {
		out = append(out, strings.TrimSpace(parts[i]))
	}
	return out, true
}

func ParseDecisionsFile(path string) ([]DecisionRow, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(raw), "\n")
	headerLine := -1
	for i, line := range lines {
		cols, ok := parseMarkdownRow(line)
		if !ok {
			continue
		}
		if len(cols) != len(decisionsHeader) {
			continue
		}
		match := true
		for j := range cols {
			if cols[j] != decisionsHeader[j] {
				match = false
				break
			}
		}
		if match {
			headerLine = i
			break
		}
	}
	if headerLine == -1 {
		return nil, fmt.Errorf("decisions table header not found in %s", path)
	}

	if headerLine+1 >= len(lines) {
		return nil, fmt.Errorf("decisions table separator missing in %s", path)
	}
	if _, ok := parseMarkdownRow(lines[headerLine+1]); !ok {
		return nil, fmt.Errorf("decisions table separator is invalid in %s", path)
	}

	rows := make([]DecisionRow, 0)
	for i := headerLine + 2; i < len(lines); i++ {
		cols, ok := parseMarkdownRow(lines[i])
		if !ok {
			if strings.TrimSpace(lines[i]) == "" {
				continue
			}
			break
		}
		if len(cols) != len(decisionsHeader) {
			return nil, fmt.Errorf("invalid decisions row column count on line %d", i+1)
		}
		rows = append(rows, DecisionRow{
			Line:              i + 1,
			DecisionID:        cols[0],
			Date:              cols[1],
			Status:            cols[2],
			PackagePath:       cols[3],
			Question:          cols[4],
			OptionsConsidered: cols[5],
			SelectedOption:    cols[6],
			Rationale:         cols[7],
			Evidence:          cols[8],
		})
	}

	return rows, nil
}
