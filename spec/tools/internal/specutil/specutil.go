package specutil

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

type DispatchRow struct {
	SlotID        string
	Status        string
	PriorityGroup string
	SpecPath      string
	SourceRoots   string
	Owner         string
	UpdatedUTC    string
	Notes         string
}

type DispatchDoc struct {
	Before string
	Rows   []DispatchRow
	After  string
}

func NowUTC() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

func IsUTCRFC3339(value string) bool {
	if value == "" {
		return false
	}
	_, err := time.Parse("2006-01-02T15:04:05Z", value)
	return err == nil
}

func ParseDispatchMarkdown(raw string) (*DispatchDoc, error) {
	startMarker := "```tsv\n"
	start := strings.Index(raw, startMarker)
	if start == -1 {
		return nil, errors.New("dispatch TSV block is missing")
	}
	end := strings.Index(raw[start+len(startMarker):], "\n```")
	if end == -1 {
		return nil, errors.New("dispatch TSV block terminator is missing")
	}
	end += start + len(startMarker)

	before := raw[:start+len(startMarker)]
	tsv := strings.TrimSpace(raw[start+len(startMarker) : end])
	after := raw[end:]

	if tsv == "" {
		return nil, errors.New("dispatch TSV is empty")
	}

	lines := strings.Split(tsv, "\n")
	header := lines[0]
	expectedHeader := "slot_id\tstatus\tpriority_group\tspec_path\tsource_roots\towner\tupdated_utc\tnotes"
	if header != expectedHeader {
		return nil, errors.New("dispatch TSV header is invalid")
	}

	rows := make([]DispatchRow, 0, len(lines)-1)
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 8 {
			return nil, fmt.Errorf("dispatch row has %d columns (expected 8): %s", len(parts), line)
		}
		rows = append(rows, DispatchRow{
			SlotID:        parts[0],
			Status:        parts[1],
			PriorityGroup: parts[2],
			SpecPath:      parts[3],
			SourceRoots:   parts[4],
			Owner:         parts[5],
			UpdatedUTC:    parts[6],
			Notes:         parts[7],
		})
	}

	return &DispatchDoc{
		Before: before,
		Rows:   rows,
		After:  after,
	}, nil
}

func ParseDispatchFile(path string) (*DispatchDoc, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseDispatchMarkdown(string(raw))
}

func RenderDispatch(doc *DispatchDoc) string {
	lines := []string{"slot_id\tstatus\tpriority_group\tspec_path\tsource_roots\towner\tupdated_utc\tnotes"}
	for _, row := range doc.Rows {
		lines = append(lines, strings.Join([]string{
			row.SlotID,
			row.Status,
			row.PriorityGroup,
			row.SpecPath,
			row.SourceRoots,
			row.Owner,
			row.UpdatedUTC,
			row.Notes,
		}, "\t"))
	}
	return doc.Before + strings.Join(lines, "\n") + doc.After
}

func FindRowBySlotID(rows []DispatchRow, slotID string) (int, *DispatchRow) {
	for i := range rows {
		if rows[i].SlotID == slotID {
			return i, &rows[i]
		}
	}
	return -1, nil
}

func ComputeSpecHash(specPath string) (string, error) {
	specFile := strings.TrimRight(specPath, "/") + "/spec.json"
	content, err := os.ReadFile(specFile)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:]), nil
}

func WithLock(lockDir string, fn func() error) error {
	for {
		err := os.Mkdir(lockDir, 0o755)
		if err == nil {
			break
		}
		if os.IsExist(err) {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		return err
	}
	defer func() {
		_ = os.Remove(lockDir)
	}()

	return fn()
}

func IsRelativeRepoPath(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	if strings.HasPrefix(value, "/") {
		return false
	}
	if strings.HasPrefix(strings.ToLower(value), "file://") {
		return false
	}
	if len(value) >= 3 {
		ch := value[0]
		if ((ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')) && value[1] == ':' && (value[2] == '\\' || value[2] == '/') {
			return false
		}
	}
	return true
}

func HasTODO(value any) bool {
	switch v := value.(type) {
	case string:
		return strings.Contains(v, "TODO")
	case []any:
		for _, item := range v {
			if HasTODO(item) {
				return true
			}
		}
	case map[string]any:
		for _, item := range v {
			if HasTODO(item) {
				return true
			}
		}
	}
	return false
}
