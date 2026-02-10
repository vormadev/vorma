package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/vormadev/vorma/spec/tools/internal/specutil"
)

type validator struct {
	slotID            string
	slotOwner         string
	specFile          string
	requireReviewPass bool
	errors            []string
	requirementIDs    map[string]struct{}
	dispatchRows      map[string]specutil.DispatchRow
}

var (
	reReqID        = regexp.MustCompile(`^REQ-[A-Z0-9-]+-[0-9]{4}$`)
	reReqPrefix    = regexp.MustCompile(`^REQ-[A-Z0-9-]+$`)
	reStatementID  = regexp.MustCompile(`^NS-[A-Z0-9-]+-[0-9]{4}$`)
	reScopeIncID   = regexp.MustCompile(`^SCB-[A-Z0-9-]+-[0-9]{4}$`)
	reScopeExID    = regexp.MustCompile(`^EXB-[A-Z0-9-]+-[0-9]{4}$`)
	reOwnedID      = regexp.MustCompile(`^OWN-[A-Z0-9-]+-[0-9]{4}$`)
	reInheritedID  = regexp.MustCompile(`^INH-[A-Z0-9-]+-[0-9]{4}$`)
	reSurfaceID    = regexp.MustCompile(`^SURF-[A-Z0-9-]+-[0-9]{4}$`)
	reCapabilityID = regexp.MustCompile(`^CAP-[A-Z0-9-]+-[0-9]{4}$`)
	reEntityID     = regexp.MustCompile(`^ENT-[A-Z0-9-]+-[0-9]{4}$`)
	reTransitionID = regexp.MustCompile(`^ST-[A-Z0-9-]+-[0-9]{4}$`)
	reErrorID      = regexp.MustCompile(`^ERR-[A-Z0-9-]+-[0-9]{4}$`)
	reContractID   = regexp.MustCompile(`^EXT-[A-Z0-9-]+-[0-9]{4}$`)
	reNFRID        = regexp.MustCompile(`^NFR-[A-Z0-9-]+-[0-9]{4}$`)
	reQuestionID   = regexp.MustCompile(`^Q-[A-Z0-9-]+-[0-9]{4}$`)
	reAssertionID  = regexp.MustCompile(`^AST-[A-Z0-9-]+-[0-9]{4}$`)
	reEvidenceID   = regexp.MustCompile(`^EVID-[A-Z0-9-]+-[0-9]{4}$`)
	reHash         = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)
	reSlot         = regexp.MustCompile(`^SLOT-[0-9]{3}$`)
	reSpecFile     = regexp.MustCompile(`(^|/)spec\.json$`)
)

func (v *validator) fail(path, msg string) {
	v.errors = append(v.errors, fmt.Sprintf("[%s] %s: %s", v.slotID, path, msg))
}

func (v *validator) asObject(path string, value any) map[string]any {
	obj, ok := value.(map[string]any)
	if !ok {
		v.fail(path, "must be an object")
		return nil
	}
	return obj
}

func (v *validator) asArray(path string, value any) []any {
	arr, ok := value.([]any)
	if !ok {
		v.fail(path, "must be an array")
		return nil
	}
	return arr
}

func (v *validator) asString(path string, value any, allowEmpty bool) string {
	s, ok := value.(string)
	if !ok {
		v.fail(path, "must be a string")
		return ""
	}
	if !allowEmpty && strings.TrimSpace(s) == "" {
		v.fail(path, "must be non-empty")
	}
	return s
}

func (v *validator) asBool(path string, value any) bool {
	b, ok := value.(bool)
	if !ok {
		v.fail(path, "must be boolean")
		return false
	}
	return b
}

func (v *validator) asInt(path string, value any, min int) int {
	switch n := value.(type) {
	case float64:
		i := int(n)
		if float64(i) != n {
			v.fail(path, "must be an integer")
			return 0
		}
		if i < min {
			v.fail(path, fmt.Sprintf("must be >= %d", min))
		}
		return i
	case json.Number:
		i64, err := n.Int64()
		if err != nil {
			v.fail(path, "must be an integer")
			return 0
		}
		i := int(i64)
		if i < min {
			v.fail(path, fmt.Sprintf("must be >= %d", min))
		}
		return i
	default:
		v.fail(path, "must be an integer")
		return 0
	}
}

func (v *validator) exactKeys(path string, obj map[string]any, required []string, optional []string) {
	if obj == nil {
		return
	}
	allowed := map[string]struct{}{}
	for _, key := range required {
		allowed[key] = struct{}{}
		if _, ok := obj[key]; !ok {
			v.fail(path, fmt.Sprintf("missing required key '%s'", key))
		}
	}
	for _, key := range optional {
		allowed[key] = struct{}{}
	}
	for key := range obj {
		if _, ok := allowed[key]; !ok {
			v.fail(path, fmt.Sprintf("unknown key '%s'", key))
		}
	}
}

func (v *validator) enum(path string, value string, allowed ...string) {
	for _, item := range allowed {
		if value == item {
			return
		}
	}
	v.fail(path, fmt.Sprintf("must be one of: %s", strings.Join(allowed, ", ")))
}

func (v *validator) pattern(path, value string, re *regexp.Regexp) {
	if value == "" {
		return
	}
	if !re.MatchString(value) {
		v.fail(path, fmt.Sprintf("invalid format: '%s'", value))
	}
}

func (v *validator) stringArray(path string, value any, min int) []string {
	arr := v.asArray(path, value)
	if arr == nil {
		return nil
	}
	if len(arr) < min {
		v.fail(path, fmt.Sprintf("must contain at least %d item(s)", min))
	}
	out := make([]string, 0, len(arr))
	for i, raw := range arr {
		out = append(out, v.asString(fmt.Sprintf("%s[%d]", path, i), raw, false))
	}
	return out
}

func (v *validator) reqRefs(path string, value any, min int, local bool) []string {
	refs := v.stringArray(path, value, min)
	for _, ref := range refs {
		v.pattern(path, ref, reReqID)
		if local {
			if _, ok := v.requirementIDs[ref]; !ok {
				v.fail(path, fmt.Sprintf("references unknown local requirement '%s'", ref))
			}
		}
	}
	return refs
}

func validateJSONFile(path string) (map[string]any, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.UseNumber()
	var data map[string]any
	if err := dec.Decode(&data); err != nil {
		return nil, err
	}
	return data, nil
}

func computeHash(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:]), nil
}

func (v *validator) validateTop(spec map[string]any) {
	v.exactKeys("spec", spec, []string{
		"meta",
		"scope",
		"capabilities",
		"requirements",
		"state_model",
		"error_model",
		"external_contracts",
		"nonfunctional",
		"open_questions",
		"assertion_accounting",
		"evidence",
		"review_gate",
	}, nil)
}

func (v *validator) validateMeta(spec map[string]any) string {
	meta := v.asObject("meta", spec["meta"])
	v.exactKeys("meta", meta, []string{"schema_version", "package", "spec_path", "source_roots", "requirement_id_prefix", "status"}, nil)
	if meta == nil {
		return ""
	}

	schemaVersion := v.asString("meta.schema_version", meta["schema_version"], false)
	if schemaVersion != "1.0.0" {
		v.fail("meta.schema_version", "must be '1.0.0'")
	}
	v.asString("meta.package", meta["package"], false)

	specPath := v.asString("meta.spec_path", meta["spec_path"], false)
	expectedPath := filepath.ToSlash(filepath.Dir(v.specFile))
	if specPath != "" && specPath != expectedPath {
		v.fail("meta.spec_path", fmt.Sprintf("must match spec file directory '%s'", expectedPath))
	}

	sourceRoots := v.stringArray("meta.source_roots", meta["source_roots"], 1)
	for i, root := range sourceRoots {
		if !specutil.IsRelativeRepoPath(root) {
			v.fail(fmt.Sprintf("meta.source_roots[%d]", i), "must be repository-relative path")
		}
	}

	reqPrefix := v.asString("meta.requirement_id_prefix", meta["requirement_id_prefix"], false)
	v.pattern("meta.requirement_id_prefix", reqPrefix, reReqPrefix)

	status := v.asString("meta.status", meta["status"], false)
	v.enum("meta.status", status, "DRAFT", "IN_PROGRESS", "COMPLETE")
	return reqPrefix
}

func (v *validator) validateRequirements(spec map[string]any, reqPrefix string) {
	requirements := v.asArray("requirements", spec["requirements"])
	if requirements == nil {
		return
	}
	if len(requirements) == 0 {
		v.fail("requirements", "must contain at least one requirement")
		return
	}

	v.requirementIDs = map[string]struct{}{}
	for i, raw := range requirements {
		path := fmt.Sprintf("requirements[%d]", i)
		req := v.asObject(path, raw)
		v.exactKeys(path, req, []string{"id", "title", "priority", "ownership", "upstream_requirement_refs", "status", "normative_statements", "rationale", "ownership_notes"}, nil)
		if req == nil {
			continue
		}

		reqID := v.asString(path+".id", req["id"], false)
		v.pattern(path+".id", reqID, reReqID)
		if reqPrefix != "" && reqID != "" && !strings.HasPrefix(reqID, reqPrefix+"-") {
			v.fail(path+".id", fmt.Sprintf("must start with '%s-'", reqPrefix))
		}
		if _, ok := v.requirementIDs[reqID]; ok {
			v.fail(path+".id", fmt.Sprintf("duplicate requirement id '%s'", reqID))
		}
		v.requirementIDs[reqID] = struct{}{}

		v.asString(path+".title", req["title"], false)
		priority := v.asString(path+".priority", req["priority"], false)
		v.enum(path+".priority", priority, "P0", "P1", "P2", "P3")

		ownership := v.asString(path+".ownership", req["ownership"], false)
		v.enum(path+".ownership", ownership, "OWNED", "INHERITED", "DELTA")

		upstream := v.reqRefs(path+".upstream_requirement_refs", req["upstream_requirement_refs"], 0, false)
		if (ownership == "INHERITED" || ownership == "DELTA") && len(upstream) == 0 {
			v.fail(path+".upstream_requirement_refs", "must contain at least 1 item for INHERITED/DELTA")
		}
		if ownership == "OWNED" && len(upstream) > 0 {
			v.fail(path+".upstream_requirement_refs", "must be empty when ownership is OWNED")
		}

		status := v.asString(path+".status", req["status"], false)
		v.enum(path+".status", status, "DRAFT", "ACTIVE", "DEPRECATED")

		normative := v.asArray(path+".normative_statements", req["normative_statements"])
		if len(normative) == 0 {
			v.fail(path+".normative_statements", "must contain at least one statement")
		}
		for j, nraw := range normative {
			nPath := fmt.Sprintf("%s.normative_statements[%d]", path, j)
			ns := v.asObject(nPath, nraw)
			v.exactKeys(nPath, ns, []string{"statement_id", "level", "text"}, nil)
			if ns == nil {
				continue
			}
			statementID := v.asString(nPath+".statement_id", ns["statement_id"], false)
			v.pattern(nPath+".statement_id", statementID, reStatementID)
			level := v.asString(nPath+".level", ns["level"], false)
			v.enum(nPath+".level", level, "MUST", "SHOULD", "MAY")
			v.asString(nPath+".text", ns["text"], false)
		}

		rationale := v.stringArray(path+".rationale", req["rationale"], 1)
		_ = rationale
		ownershipNotes := v.stringArray(path+".ownership_notes", req["ownership_notes"], 0)
		_ = ownershipNotes
	}
}

func (v *validator) validateScope(spec map[string]any) {
	scope := v.asObject("scope", spec["scope"])
	v.exactKeys("scope", scope, []string{"included_behaviors", "excluded_behaviors", "ownership_boundaries", "public_surfaces"}, nil)
	if scope == nil {
		return
	}

	included := v.asArray("scope.included_behaviors", scope["included_behaviors"])
	for i, raw := range included {
		path := fmt.Sprintf("scope.included_behaviors[%d]", i)
		item := v.asObject(path, raw)
		v.exactKeys(path, item, []string{"behavior_id", "statement", "requirement_refs"}, nil)
		if item == nil {
			continue
		}
		id := v.asString(path+".behavior_id", item["behavior_id"], false)
		v.pattern(path+".behavior_id", id, reScopeIncID)
		v.asString(path+".statement", item["statement"], false)
		v.reqRefs(path+".requirement_refs", item["requirement_refs"], 1, true)
	}

	excluded := v.asArray("scope.excluded_behaviors", scope["excluded_behaviors"])
	for i, raw := range excluded {
		path := fmt.Sprintf("scope.excluded_behaviors[%d]", i)
		item := v.asObject(path, raw)
		v.exactKeys(path, item, []string{"behavior_id", "reason"}, nil)
		if item == nil {
			continue
		}
		id := v.asString(path+".behavior_id", item["behavior_id"], false)
		v.pattern(path+".behavior_id", id, reScopeExID)
		v.asString(path+".reason", item["reason"], false)
	}

	boundaries := v.asObject("scope.ownership_boundaries", scope["ownership_boundaries"])
	v.exactKeys("scope.ownership_boundaries", boundaries, []string{"owned_behaviors", "inherited_behaviors"}, nil)
	if boundaries != nil {
		owned := v.asArray("scope.ownership_boundaries.owned_behaviors", boundaries["owned_behaviors"])
		for i, raw := range owned {
			path := fmt.Sprintf("scope.ownership_boundaries.owned_behaviors[%d]", i)
			item := v.asObject(path, raw)
			v.exactKeys(path, item, []string{"behavior_id", "statement", "requirement_refs"}, nil)
			if item == nil {
				continue
			}
			id := v.asString(path+".behavior_id", item["behavior_id"], false)
			v.pattern(path+".behavior_id", id, reOwnedID)
			v.asString(path+".statement", item["statement"], false)
			v.reqRefs(path+".requirement_refs", item["requirement_refs"], 1, true)
		}

		inherited := v.asArray("scope.ownership_boundaries.inherited_behaviors", boundaries["inherited_behaviors"])
		for i, raw := range inherited {
			path := fmt.Sprintf("scope.ownership_boundaries.inherited_behaviors[%d]", i)
			item := v.asObject(path, raw)
			v.exactKeys(path, item, []string{"behavior_id", "statement", "upstream_requirement_refs", "local_delta_notes", "requirement_refs"}, nil)
			if item == nil {
				continue
			}
			id := v.asString(path+".behavior_id", item["behavior_id"], false)
			v.pattern(path+".behavior_id", id, reInheritedID)
			v.asString(path+".statement", item["statement"], false)
			v.reqRefs(path+".upstream_requirement_refs", item["upstream_requirement_refs"], 1, false)
			v.stringArray(path+".local_delta_notes", item["local_delta_notes"], 0)
			v.reqRefs(path+".requirement_refs", item["requirement_refs"], 1, true)
		}
	}

	publicSurfaces := v.asArray("scope.public_surfaces", scope["public_surfaces"])
	for i, raw := range publicSurfaces {
		path := fmt.Sprintf("scope.public_surfaces[%d]", i)
		item := v.asObject(path, raw)
		v.exactKeys(path, item, []string{"surface_id", "kind", "symbol_or_path", "owner_package", "stability", "requirement_refs", "notes"}, nil)
		if item == nil {
			continue
		}
		surfaceID := v.asString(path+".surface_id", item["surface_id"], false)
		v.pattern(path+".surface_id", surfaceID, reSurfaceID)
		kind := v.asString(path+".kind", item["kind"], false)
		v.enum(path+".kind", kind, "function", "type", "http_route", "cli", "env", "filesystem", "other")
		v.asString(path+".symbol_or_path", item["symbol_or_path"], false)
		v.asString(path+".owner_package", item["owner_package"], false)
		stability := v.asString(path+".stability", item["stability"], false)
		v.enum(path+".stability", stability, "STABLE", "EXPERIMENTAL", "DEPRECATED")
		v.reqRefs(path+".requirement_refs", item["requirement_refs"], 1, true)
		v.stringArray(path+".notes", item["notes"], 0)
	}
}

func (v *validator) validateCapabilities(spec map[string]any) {
	caps := v.asArray("capabilities", spec["capabilities"])
	for i, raw := range caps {
		path := fmt.Sprintf("capabilities[%d]", i)
		cap := v.asObject(path, raw)
		v.exactKeys(path, cap, []string{"capability_id", "title", "summary", "inputs", "outputs", "side_effects", "requirement_refs"}, nil)
		if cap == nil {
			continue
		}

		id := v.asString(path+".capability_id", cap["capability_id"], false)
		v.pattern(path+".capability_id", id, reCapabilityID)
		v.asString(path+".title", cap["title"], false)
		v.asString(path+".summary", cap["summary"], false)

		inputs := v.asArray(path+".inputs", cap["inputs"])
		for j, inputRaw := range inputs {
			ipath := fmt.Sprintf("%s.inputs[%d]", path, j)
			input := v.asObject(ipath, inputRaw)
			v.exactKeys(ipath, input, []string{"name", "type", "required", "description"}, nil)
			if input == nil {
				continue
			}
			v.asString(ipath+".name", input["name"], false)
			v.asString(ipath+".type", input["type"], false)
			v.asBool(ipath+".required", input["required"])
			v.asString(ipath+".description", input["description"], false)
		}

		outputs := v.asArray(path+".outputs", cap["outputs"])
		for j, outputRaw := range outputs {
			opath := fmt.Sprintf("%s.outputs[%d]", path, j)
			output := v.asObject(opath, outputRaw)
			v.exactKeys(opath, output, []string{"name", "type", "description"}, nil)
			if output == nil {
				continue
			}
			v.asString(opath+".name", output["name"], false)
			v.asString(opath+".type", output["type"], false)
			v.asString(opath+".description", output["description"], false)
		}

		v.stringArray(path+".side_effects", cap["side_effects"], 0)
		v.reqRefs(path+".requirement_refs", cap["requirement_refs"], 1, true)
	}
}

func (v *validator) validateStateModel(spec map[string]any) {
	stateModel := v.asObject("state_model", spec["state_model"])
	v.exactKeys("state_model", stateModel, []string{"entities", "transitions"}, nil)
	if stateModel == nil {
		return
	}

	entities := v.asArray("state_model.entities", stateModel["entities"])
	for i, raw := range entities {
		path := fmt.Sprintf("state_model.entities[%d]", i)
		entity := v.asObject(path, raw)
		v.exactKeys(path, entity, []string{"entity_id", "description", "fields"}, nil)
		if entity == nil {
			continue
		}
		id := v.asString(path+".entity_id", entity["entity_id"], false)
		v.pattern(path+".entity_id", id, reEntityID)
		v.asString(path+".description", entity["description"], false)
		fields := v.asArray(path+".fields", entity["fields"])
		for j, fRaw := range fields {
			fPath := fmt.Sprintf("%s.fields[%d]", path, j)
			field := v.asObject(fPath, fRaw)
			v.exactKeys(fPath, field, []string{"name", "type", "nullable", "description"}, nil)
			if field == nil {
				continue
			}
			v.asString(fPath+".name", field["name"], false)
			v.asString(fPath+".type", field["type"], false)
			v.asBool(fPath+".nullable", field["nullable"])
			v.asString(fPath+".description", field["description"], false)
		}
	}

	transitions := v.asArray("state_model.transitions", stateModel["transitions"])
	for i, raw := range transitions {
		path := fmt.Sprintf("state_model.transitions[%d]", i)
		tr := v.asObject(path, raw)
		v.exactKeys(path, tr, []string{"transition_id", "from", "to", "trigger", "guard_conditions", "effects", "requirement_refs"}, nil)
		if tr == nil {
			continue
		}
		id := v.asString(path+".transition_id", tr["transition_id"], false)
		v.pattern(path+".transition_id", id, reTransitionID)
		v.asString(path+".from", tr["from"], false)
		v.asString(path+".to", tr["to"], false)
		v.asString(path+".trigger", tr["trigger"], false)
		v.stringArray(path+".guard_conditions", tr["guard_conditions"], 0)
		v.stringArray(path+".effects", tr["effects"], 0)
		v.reqRefs(path+".requirement_refs", tr["requirement_refs"], 1, true)
	}
}

func (v *validator) validateErrorModel(spec map[string]any) {
	errorModel := v.asObject("error_model", spec["error_model"])
	v.exactKeys("error_model", errorModel, []string{"errors"}, nil)
	if errorModel == nil {
		return
	}

	errs := v.asArray("error_model.errors", errorModel["errors"])
	for i, raw := range errs {
		path := fmt.Sprintf("error_model.errors[%d]", i)
		err := v.asObject(path, raw)
		v.exactKeys(path, err, []string{"error_id", "condition", "surface", "error_code", "required_behavior", "requirement_refs"}, nil)
		if err == nil {
			continue
		}
		id := v.asString(path+".error_id", err["error_id"], false)
		v.pattern(path+".error_id", id, reErrorID)
		v.asString(path+".condition", err["condition"], false)
		v.asString(path+".surface", err["surface"], false)
		v.asString(path+".error_code", err["error_code"], false)
		v.stringArray(path+".required_behavior", err["required_behavior"], 1)
		v.reqRefs(path+".requirement_refs", err["requirement_refs"], 1, true)
	}
}

func (v *validator) validateExternalContracts(spec map[string]any) {
	ext := v.asObject("external_contracts", spec["external_contracts"])
	v.exactKeys("external_contracts", ext, []string{"contracts"}, nil)
	if ext == nil {
		return
	}

	contracts := v.asArray("external_contracts.contracts", ext["contracts"])
	for i, raw := range contracts {
		path := fmt.Sprintf("external_contracts.contracts[%d]", i)
		c := v.asObject(path, raw)
		v.exactKeys(path, c, []string{"contract_id", "type", "interface", "versioning_and_compatibility_rules", "requirement_refs"}, nil)
		if c == nil {
			continue
		}
		id := v.asString(path+".contract_id", c["contract_id"], false)
		v.pattern(path+".contract_id", id, reContractID)
		typeValue := v.asString(path+".type", c["type"], false)
		v.enum(path+".type", typeValue, "protocol", "schema", "cli", "api", "filesystem", "env", "other")
		v.asString(path+".interface", c["interface"], false)
		v.stringArray(path+".versioning_and_compatibility_rules", c["versioning_and_compatibility_rules"], 1)
		v.reqRefs(path+".requirement_refs", c["requirement_refs"], 1, true)
	}
}

func (v *validator) validateNonFunctional(spec map[string]any) {
	nf := v.asObject("nonfunctional", spec["nonfunctional"])
	v.exactKeys("nonfunctional", nf, []string{"requirements"}, nil)
	if nf == nil {
		return
	}

	reqs := v.asArray("nonfunctional.requirements", nf["requirements"])
	for i, raw := range reqs {
		path := fmt.Sprintf("nonfunctional.requirements[%d]", i)
		r := v.asObject(path, raw)
		v.exactKeys(path, r, []string{"nfr_id", "category", "requirement", "measurement_or_evidence_notes", "requirement_refs"}, nil)
		if r == nil {
			continue
		}
		id := v.asString(path+".nfr_id", r["nfr_id"], false)
		v.pattern(path+".nfr_id", id, reNFRID)
		cat := v.asString(path+".category", r["category"], false)
		v.enum(path+".category", cat, "performance", "security", "reliability", "compatibility", "operability", "other")
		v.asString(path+".requirement", r["requirement"], false)
		v.stringArray(path+".measurement_or_evidence_notes", r["measurement_or_evidence_notes"], 0)
		v.reqRefs(path+".requirement_refs", r["requirement_refs"], 1, true)
	}
}

func (v *validator) validateOpenQuestions(spec map[string]any) {
	open := v.asObject("open_questions", spec["open_questions"])
	v.exactKeys("open_questions", open, []string{"questions"}, nil)
	if open == nil {
		return
	}

	questions := v.asArray("open_questions.questions", open["questions"])
	for i, raw := range questions {
		path := fmt.Sprintf("open_questions.questions[%d]", i)
		q := v.asObject(path, raw)
		v.exactKeys(path, q, []string{"question_id", "question", "status", "owner", "resolution", "notes"}, nil)
		if q == nil {
			continue
		}
		qid := v.asString(path+".question_id", q["question_id"], false)
		v.pattern(path+".question_id", qid, reQuestionID)
		v.asString(path+".question", q["question"], false)
		status := v.asString(path+".status", q["status"], false)
		v.enum(path+".status", status, "OPEN", "RESOLVED")
		v.asString(path+".owner", q["owner"], false)
		resolution := v.asString(path+".resolution", q["resolution"], true)
		if status == "RESOLVED" && strings.TrimSpace(resolution) == "" {
			v.fail(path+".resolution", "must be non-empty when status is RESOLVED")
		}
		if v.requireReviewPass && status != "RESOLVED" {
			v.fail(path, "open question must be RESOLVED when review pass is required")
		}
		v.stringArray(path+".notes", q["notes"], 0)
	}
}

func (v *validator) validateAssertionAccounting(spec map[string]any) {
	acct := v.asObject("assertion_accounting", spec["assertion_accounting"])
	v.exactKeys("assertion_accounting", acct, []string{"ledger", "non_meaningful_exclusion_notes"}, nil)
	if acct == nil {
		return
	}
	v.stringArray("assertion_accounting.non_meaningful_exclusion_notes", acct["non_meaningful_exclusion_notes"], 0)

	ledger := v.asArray("assertion_accounting.ledger", acct["ledger"])
	if ledger == nil {
		return
	}
	if len(ledger) == 0 {
		v.fail("assertion_accounting.ledger", "must contain at least one assertion entry")
	}

	assertionIDs := map[string]struct{}{}
	meaningful := 0
	nonMeaningful := 0

	for i, raw := range ledger {
		path := fmt.Sprintf("assertion_accounting.ledger[%d]", i)
		row := v.asObject(path, raw)
		v.exactKeys(path, row, []string{"assertion_id", "source_test_ref", "disposition", "requirement_ids", "rationale"}, nil)
		if row == nil {
			continue
		}
		assertionID := v.asString(path+".assertion_id", row["assertion_id"], false)
		v.pattern(path+".assertion_id", assertionID, reAssertionID)
		if _, ok := assertionIDs[assertionID]; ok {
			v.fail(path+".assertion_id", fmt.Sprintf("duplicate assertion id '%s'", assertionID))
		}
		assertionIDs[assertionID] = struct{}{}

		source := v.asObject(path+".source_test_ref", row["source_test_ref"])
		v.exactKeys(path+".source_test_ref", source, []string{"path", "line"}, nil)
		if source != nil {
			sourcePath := v.asString(path+".source_test_ref.path", source["path"], false)
			if !specutil.IsRelativeRepoPath(sourcePath) {
				v.fail(path+".source_test_ref.path", "must be repository-relative path")
			}
			v.asInt(path+".source_test_ref.line", source["line"], 1)
		}

		disp := v.asString(path+".disposition", row["disposition"], false)
		v.enum(path+".disposition", disp, "MEANINGFUL", "NON_MEANINGFUL")
		reqIDs := v.reqRefs(path+".requirement_ids", row["requirement_ids"], 0, false)
		if disp == "MEANINGFUL" {
			if len(reqIDs) == 0 {
				v.fail(path+".requirement_ids", "must contain at least 1 item for MEANINGFUL assertion")
			}
			for _, reqID := range reqIDs {
				if _, ok := v.requirementIDs[reqID]; !ok {
					v.fail(path+".requirement_ids", fmt.Sprintf("references unknown local requirement '%s'", reqID))
				}
			}
			meaningful++
		}
		if disp == "NON_MEANINGFUL" {
			if len(reqIDs) > 0 {
				v.fail(path+".requirement_ids", "must be empty for NON_MEANINGFUL assertion")
			}
			nonMeaningful++
		}

		v.asString(path+".rationale", row["rationale"], false)
	}

	unclassified := len(ledger) - meaningful - nonMeaningful
	if unclassified != 0 {
		v.fail("assertion_accounting.ledger", fmt.Sprintf("derived unclassified assertion count must be 0, got %d", unclassified))
	}
}

func (v *validator) validateEvidence(spec map[string]any) {
	evidence := v.asObject("evidence", spec["evidence"])
	v.exactKeys("evidence", evidence, []string{"requirements"}, nil)
	if evidence == nil {
		return
	}
	mapObj := v.asObject("evidence.requirements", evidence["requirements"])
	if mapObj == nil {
		return
	}

	for reqID := range mapObj {
		v.pattern("evidence.requirements", reqID, reReqID)
		if _, ok := v.requirementIDs[reqID]; !ok {
			v.fail("evidence.requirements", fmt.Sprintf("has mapping for unknown requirement '%s'", reqID))
		}
	}

	reqIDs := make([]string, 0, len(v.requirementIDs))
	for reqID := range v.requirementIDs {
		reqIDs = append(reqIDs, reqID)
	}
	sort.Strings(reqIDs)

	for _, reqID := range reqIDs {
		raw, ok := mapObj[reqID]
		if !ok {
			v.fail("evidence.requirements", fmt.Sprintf("missing evidence mapping for '%s'", reqID))
			continue
		}
		path := "evidence.requirements." + reqID
		entry := v.asObject(path, raw)
		v.exactKeys(path, entry, []string{"evidences"}, nil)
		if entry == nil {
			continue
		}

		evidences := v.asArray(path+".evidences", entry["evidences"])
		if len(evidences) == 0 {
			v.fail(path+".evidences", "must contain at least one entry")
		}
		kinds := map[string]bool{}
		evidenceIDs := map[string]struct{}{}

		for i, eraw := range evidences {
			ePath := fmt.Sprintf("%s.evidences[%d]", path, i)
			e := v.asObject(ePath, eraw)
			v.exactKeys(ePath, e, []string{"evidence_id", "kind", "refs", "note"}, nil)
			if e == nil {
				continue
			}

			eID := v.asString(ePath+".evidence_id", e["evidence_id"], false)
			v.pattern(ePath+".evidence_id", eID, reEvidenceID)
			if _, ok := evidenceIDs[eID]; ok {
				v.fail(ePath+".evidence_id", fmt.Sprintf("duplicate evidence_id '%s' for %s", eID, reqID))
			}
			evidenceIDs[eID] = struct{}{}

			kind := v.asString(ePath+".kind", e["kind"], false)
			v.enum(ePath+".kind", kind, "test", "implementation")
			kinds[kind] = true

			refs := v.asArray(ePath+".refs", e["refs"])
			if len(refs) == 0 {
				v.fail(ePath+".refs", "must contain at least one reference")
			}
			for j, rraw := range refs {
				rPath := fmt.Sprintf("%s.refs[%d]", ePath, j)
				r := v.asObject(rPath, rraw)
				v.exactKeys(rPath, r, []string{"path", "line"}, []string{"column", "symbol"})
				if r == nil {
					continue
				}
				pathValue := v.asString(rPath+".path", r["path"], false)
				if !specutil.IsRelativeRepoPath(pathValue) {
					v.fail(rPath+".path", "must be repository-relative path")
				}
				v.asInt(rPath+".line", r["line"], 1)
				if col, ok := r["column"]; ok {
					v.asInt(rPath+".column", col, 1)
				}
				if sym, ok := r["symbol"]; ok {
					v.asString(rPath+".symbol", sym, false)
				}
			}

			v.asString(ePath+".note", e["note"], false)
		}

		if !kinds["test"] {
			v.fail(path+".evidences", fmt.Sprintf("missing test evidence for '%s'", reqID))
		}
		if !kinds["implementation"] {
			v.fail(path+".evidences", fmt.Sprintf("missing implementation evidence for '%s'", reqID))
		}
	}
}

type reviewPass struct {
	ReviewerOwner     string
	ReviewerClaimSlot string
	Result            string
	ArtifactsHash     string
	CompletedUTC      string
	NotesRef          string
	NotesLogLen       int
}

func (v *validator) validateReviewPass(path string, value any) *reviewPass {
	passObj := v.asObject(path, value)
	v.exactKeys(path, passObj, []string{"reviewer_owner", "reviewer_claim_slot", "result", "artifacts_hash", "completed_utc", "notes_ref", "notes_log"}, nil)
	if passObj == nil {
		return nil
	}

	reviewerOwner := v.asString(path+".reviewer_owner", passObj["reviewer_owner"], false)
	reviewerClaimSlot := v.asString(path+".reviewer_claim_slot", passObj["reviewer_claim_slot"], false)
	result := v.asString(path+".result", passObj["result"], false)
	v.enum(path+".result", result, "PENDING", "PASS_NO_NOTES", "FAIL_NOTES")
	artifactsHash := v.asString(path+".artifacts_hash", passObj["artifacts_hash"], false)
	completedUTC := v.asString(path+".completed_utc", passObj["completed_utc"], false)
	notesRef := v.asString(path+".notes_ref", passObj["notes_ref"], false)

	if artifactsHash != "-" && !reHash.MatchString(artifactsHash) {
		v.fail(path+".artifacts_hash", "must be '-' or SHA256 hex")
	}
	if completedUTC != "-" && !specutil.IsUTCRFC3339(completedUTC) {
		v.fail(path+".completed_utc", "must be '-' or UTC RFC3339 timestamp")
	}
	if reviewerClaimSlot != "-" && !reSlot.MatchString(reviewerClaimSlot) {
		v.fail(path+".reviewer_claim_slot", "must be '-' or SLOT-XXX")
	}

	notesLog := v.asArray(path+".notes_log", passObj["notes_log"])
	for i, raw := range notesLog {
		logPath := fmt.Sprintf("%s.notes_log[%d]", path, i)
		entry := v.asObject(logPath, raw)
		v.exactKeys(logPath, entry, []string{"at_utc", "notes_ref", "comment"}, nil)
		if entry == nil {
			continue
		}
		atUTC := v.asString(logPath+".at_utc", entry["at_utc"], false)
		if !specutil.IsUTCRFC3339(atUTC) {
			v.fail(logPath+".at_utc", "must be UTC RFC3339 timestamp")
		}
		v.asString(logPath+".notes_ref", entry["notes_ref"], false)
		v.asString(logPath+".comment", entry["comment"], false)
	}

	if result == "PENDING" {
		if reviewerOwner != "-" || reviewerClaimSlot != "-" || artifactsHash != "-" || completedUTC != "-" || notesRef != "-" {
			v.fail(path, "PENDING pass must keep reviewer_owner/reviewer_claim_slot/artifacts_hash/completed_utc/notes_ref as '-' ")
		}
	}
	if result == "PASS_NO_NOTES" && notesRef != "-" {
		v.fail(path+".notes_ref", "must be '-' when result is PASS_NO_NOTES")
	}
	if result == "FAIL_NOTES" && notesRef == "-" {
		v.fail(path+".notes_ref", "must not be '-' when result is FAIL_NOTES")
	}

	return &reviewPass{
		ReviewerOwner:     reviewerOwner,
		ReviewerClaimSlot: reviewerClaimSlot,
		Result:            result,
		ArtifactsHash:     artifactsHash,
		CompletedUTC:      completedUTC,
		NotesRef:          notesRef,
		NotesLogLen:       len(notesLog),
	}
}

func (v *validator) validateReviewGate(spec map[string]any) {
	review := v.asObject("review_gate", spec["review_gate"])
	v.exactKeys("review_gate", review, []string{"miner_owner", "target_artifacts_hash", "pass1", "pass2"}, nil)
	if review == nil {
		return
	}

	minerOwner := v.asString("review_gate.miner_owner", review["miner_owner"], false)
	targetHash := v.asString("review_gate.target_artifacts_hash", review["target_artifacts_hash"], false)
	if targetHash != "-" && !reHash.MatchString(targetHash) {
		v.fail("review_gate.target_artifacts_hash", "must be '-' or SHA256 hex")
	}
	p1 := v.validateReviewPass("review_gate.pass1", review["pass1"])
	p2 := v.validateReviewPass("review_gate.pass2", review["pass2"])

	if !v.requireReviewPass {
		return
	}

	meta := v.asObject("meta", spec["meta"])
	if meta != nil {
		status := v.asString("meta.status", meta["status"], false)
		if status != "COMPLETE" {
			v.fail("meta.status", "must be COMPLETE when review pass is required")
		}
	}

	currentHash, err := computeHash(v.specFile)
	if err != nil {
		v.fail("spec", fmt.Sprintf("failed to hash spec file: %v", err))
		return
	}

	if minerOwner != v.slotOwner {
		v.fail("review_gate.miner_owner", fmt.Sprintf("must match slot owner '%s'", v.slotOwner))
	}
	if targetHash != currentHash {
		v.fail("review_gate.target_artifacts_hash", fmt.Sprintf("must match current artifact hash '%s'", currentHash))
	}
	if p1 == nil || p2 == nil {
		return
	}

	if p1.Result != "PASS_NO_NOTES" || p2.Result != "PASS_NO_NOTES" {
		v.fail("review_gate", "both passes must be PASS_NO_NOTES")
	}

	if p1.ReviewerOwner == "-" || p2.ReviewerOwner == "-" {
		v.fail("review_gate", "reviewer_owner must be set for both passes")
	}
	if p1.ReviewerOwner == v.slotOwner || p2.ReviewerOwner == v.slotOwner {
		v.fail("review_gate", "reviewers must be independent from miner owner")
	}
	if p1.ReviewerOwner == p2.ReviewerOwner {
		v.fail("review_gate", "pass1 and pass2 reviewer_owner must differ")
	}

	if p1.ReviewerClaimSlot == "-" || p2.ReviewerClaimSlot == "-" {
		v.fail("review_gate", "reviewer_claim_slot must be set for both passes")
	}
	if p1.ReviewerClaimSlot == v.slotID || p2.ReviewerClaimSlot == v.slotID {
		v.fail("review_gate", "reviewer_claim_slot must differ from mined slot")
	}
	if p1.ReviewerClaimSlot == p2.ReviewerClaimSlot {
		v.fail("review_gate", "pass1 and pass2 reviewer_claim_slot must differ")
	}

	checkClaim := func(passPath string, pass *reviewPass) {
		row, ok := v.dispatchRows[pass.ReviewerClaimSlot]
		if !ok {
			v.fail(passPath+".reviewer_claim_slot", fmt.Sprintf("slot '%s' not found in dispatch", pass.ReviewerClaimSlot))
			return
		}
		if row.Owner != pass.ReviewerOwner {
			v.fail(passPath, fmt.Sprintf("reviewer_owner '%s' does not match dispatch owner '%s'", pass.ReviewerOwner, row.Owner))
		}
		if row.Status != "CLAIMED" && row.Status != "DONE" {
			v.fail(passPath, fmt.Sprintf("reviewer claim slot must be CLAIMED or DONE, got '%s'", row.Status))
		}
	}
	checkClaim("review_gate.pass1", p1)
	checkClaim("review_gate.pass2", p2)

	if p1.ArtifactsHash != currentHash || p2.ArtifactsHash != currentHash {
		v.fail("review_gate", fmt.Sprintf("review pass artifact hashes must match current hash '%s'", currentHash))
	}
	if !specutil.IsUTCRFC3339(p1.CompletedUTC) || !specutil.IsUTCRFC3339(p2.CompletedUTC) {
		v.fail("review_gate", "review pass completed_utc must be UTC RFC3339 timestamps")
	}
	if p1.NotesRef != "-" || p2.NotesRef != "-" {
		v.fail("review_gate", "PASS_NO_NOTES requires notes_ref '-' for both passes")
	}
	if p1.NotesLogLen != 0 || p2.NotesLogLen != 0 {
		v.fail("review_gate", "PASS_NO_NOTES requires notes_log to be empty for both passes")
	}
}

func main() {
	slot := flag.String("slot", "", "SLOT-XXX")
	specFile := flag.String("spec", "", "spec/packages/<path>/spec.json")
	owner := flag.String("owner", "", "slot owner")
	dispatch := flag.String("dispatch", "", "dispatch markdown file")
	requireReviewPass := flag.Bool("require-review-pass", false, "enforce review pass fields")
	flag.Parse()

	if *slot == "" || *specFile == "" || *owner == "" || *dispatch == "" {
		fmt.Fprintln(os.Stderr, "usage: go run ./spec/tools/cmd/validate_slot_spec --slot SLOT-XXX --spec spec/packages/<path>/spec.json --owner <owner> --dispatch spec/MINING_DISPATCH.md [--require-review-pass]")
		os.Exit(2)
	}
	if !reSpecFile.MatchString(filepath.ToSlash(*specFile)) {
		fmt.Fprintln(os.Stderr, "spec path must target spec.json")
		os.Exit(2)
	}

	doc, err := specutil.ParseDispatchFile(*dispatch)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(2)
	}

	rows := map[string]specutil.DispatchRow{}
	for _, row := range doc.Rows {
		rows[row.SlotID] = row
	}

	spec, err := validateJSONFile(*specFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[%s] spec: invalid JSON: %v\n", *slot, err)
		os.Exit(1)
	}

	v := &validator{
		slotID:            *slot,
		slotOwner:         *owner,
		specFile:          filepath.ToSlash(*specFile),
		requireReviewPass: *requireReviewPass,
		requirementIDs:    map[string]struct{}{},
		dispatchRows:      rows,
	}

	if specutil.HasTODO(spec) {
		v.fail("spec", "TODO markers remain")
	}

	v.validateTop(spec)
	reqPrefix := v.validateMeta(spec)
	v.validateRequirements(spec, reqPrefix)
	v.validateScope(spec)
	v.validateCapabilities(spec)
	v.validateStateModel(spec)
	v.validateErrorModel(spec)
	v.validateExternalContracts(spec)
	v.validateNonFunctional(spec)
	v.validateOpenQuestions(spec)
	v.validateAssertionAccounting(spec)
	v.validateEvidence(spec)
	v.validateReviewGate(spec)

	if len(v.errors) > 0 {
		for _, e := range v.errors {
			fmt.Fprintln(os.Stderr, e)
		}
		os.Exit(1)
	}
}
