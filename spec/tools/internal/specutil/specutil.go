package specutil

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type DispatchRow struct {
	SlotID        string `json:"slot_id"`
	Status        string `json:"status"`
	PriorityGroup string `json:"priority_group"`
	SpecPath      string `json:"spec_path"`
	SourceRoots   string `json:"source_roots"`
	Owner         string `json:"owner"`
	UpdatedUTC    string `json:"updated_utc"`
	Notes         string `json:"notes"`
	ClaimBranch   string `json:"claim_branch"`
	ClaimContext  string `json:"claim_context"`
	ClaimActor    string `json:"claim_actor"`
}

type DispatchDoc struct {
	SchemaVersion string        `json:"schema_version"`
	Rows          []DispatchRow `json:"slots"`
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

func ParseDispatchFile(path string) (*DispatchDoc, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	doc, err := ParseDispatchJSON(string(raw))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return doc, nil
}

func ParseDispatchJSON(raw string) (*DispatchDoc, error) {
	doc := DispatchDoc{}
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return nil, fmt.Errorf("invalid dispatch JSON: %w", err)
	}
	if doc.SchemaVersion != "1.0.0" {
		return nil, fmt.Errorf("invalid dispatch schema_version: %s", doc.SchemaVersion)
	}
	if len(doc.Rows) == 0 {
		return nil, fmt.Errorf("dispatch has no slots")
	}
	seen := map[string]struct{}{}
	for i := range doc.Rows {
		row := &doc.Rows[i]
		if row.SlotID == "" {
			return nil, fmt.Errorf("dispatch row has empty slot_id")
		}
		if _, ok := seen[row.SlotID]; ok {
			return nil, fmt.Errorf("duplicate dispatch slot_id: %s", row.SlotID)
		}
		seen[row.SlotID] = struct{}{}
		if strings.TrimSpace(row.ClaimBranch) == "" {
			row.ClaimBranch = "-"
		}
		if strings.TrimSpace(row.ClaimContext) == "" {
			row.ClaimContext = "-"
		}
		if strings.TrimSpace(row.ClaimActor) == "" {
			row.ClaimActor = "-"
		}
	}
	return &doc, nil
}

func RenderDispatch(doc *DispatchDoc) string {
	encoded, err := json.MarshalIndent(doc, "", "\t")
	if err != nil {
		panic(err)
	}
	return string(encoded) + "\n"
}

func LoadDispatchState(localPath string) (*DispatchDoc, error) {
	statePath, err := DispatchStatePath()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(statePath); err == nil {
		return ParseDispatchFile(statePath)
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	doc, err := ParseDispatchFile(localPath)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(statePath, []byte(RenderDispatch(doc)), 0o644); err != nil {
		return nil, err
	}
	return doc, nil
}

func ReadDispatchState(localPath string) (*DispatchDoc, error) {
	statePath, err := DispatchStatePath()
	if err != nil {
		return ParseDispatchFile(localPath)
	}
	if _, err := os.Stat(statePath); err == nil {
		return ParseDispatchFile(statePath)
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return ParseDispatchFile(localPath)
}

func SaveDispatchState(localPath string, doc *DispatchDoc) error {
	rendered := RenderDispatch(doc)
	statePath, err := DispatchStatePath()
	if err != nil {
		return err
	}
	if err := os.WriteFile(statePath, []byte(rendered), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(localPath, []byte(rendered), 0o644); err != nil {
		return err
	}
	return nil
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
	specFile := specPath + "/spec.json"
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
	if value[0] == '/' {
		return false
	}
	if len(value) >= 7 && (value[:7] == "file://" || value[:7] == "FILE://") {
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
