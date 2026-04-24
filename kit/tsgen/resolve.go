package tsgen

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

/////////////////////////////////////////////////////////////////////
/////// Public API
/////////////////////////////////////////////////////////////////////

// ID is a deterministic, opaque identifier for a resolved type.
type ID string

// GoTypeSrc represents a Go type that needs a TypeScript definition.
type GoTypeSrc struct {
	Instance      any    // Required
	RequestedName string // Optional
}

func GoType[T any](requested_name ...string) *GoTypeSrc {
	var zero T
	n := ""
	if len(requested_name) > 0 {
		n = requested_name[0]
	}
	return &GoTypeSrc{Instance: zero, RequestedName: n}
}

// ID returns the deterministic ID for this type.
// This is the same key used in the map returned by Registry.ResolveTypes().
// It can also be used as a TSTyper value (via string(id)) to create
// a reference to a system-managed type.
func (a GoTypeSrc) ID() ID {
	t := effective_reflect_type(a.Instance)
	eff_name := effective_requested_name(t, a.RequestedName)
	return ID(make_id(t, eff_name))
}

// ResolvedTSType is a single resolved type definition.
type ResolvedTSType struct {
	ID   ID
	Name string // Resolved, unique TS name (e.g. "UserResponse")
	Body string // The TS type body (e.g. "{ id: string; name: string }")
}

// GoTypeRegistry collects Go types and resolves them into TypeScript definitions.
type GoTypeRegistry struct {
	inputs []*GoTypeSrc
}

// Add registers one or more Go types for TypeScript generation.
func (r *GoTypeRegistry) Add(input_types ...*GoTypeSrc) {
	r.inputs = append(r.inputs, input_types...)
}

type ResolvedTSTypes map[ID]ResolvedTSType

// ResolveTypes walks all registered types, deduplicates, resolves name
// collisions, and returns the final map of type definitions keyed by ID.
func (r *GoTypeRegistry) ResolveTypes() (ResolvedTSTypes, error) {
	all := make([]map[string]*walk_entry, 0, len(r.inputs))
	for _, input := range r.inputs {
		if input == nil {
			continue
		}
		entries, _ := walk_type(input.Instance, input.RequestedName)
		if entries != nil {
			all = append(all, entries)
		}
	}
	return merge_and_resolve(all), nil
}

/////////////////////////////////////////////////////////////////////
/////// Merge, deduplicate, resolve names
/////////////////////////////////////////////////////////////////////

func merge_and_resolve(all []map[string]*walk_entry) ResolvedTSTypes {
	if len(all) == 0 {
		return nil
	}

	// Flatten with dedup — merge flags for entries with the same id.
	flat := make(map[string]*walk_entry)
	for _, entries := range all {
		for id, entry := range entries {
			if existing, ok := flat[id]; ok {
				existing.is_root = existing.is_root || entry.is_root
				existing.is_referenced = existing.is_referenced ||
					entry.is_referenced
				existing.used_as_embedded = existing.used_as_embedded ||
					entry.used_as_embedded
			} else {
				flat[id] = entry
			}
		}
	}

	// Collect all ids that are referenced via kind_ref or via sentinel
	// strings embedded in kind_raw nodes (e.g. from TSTyper).
	ref_ids := collect_all_ref_ids(flat)

	// Keep entries that are roots, referenced as fields, or pointed to
	// by a ref node or sentinel string.
	var keep_ids []string
	name_count := make(map[string]int)
	for id, entry := range flat {
		if entry.is_root || entry.is_referenced || ref_ids[id] {
			keep_ids = append(keep_ids, id)
			if entry.requested_name != "" {
				name_count[entry.requested_name]++
			}
		}
	}
	slices.Sort(keep_ids)

	// Resolve name collisions by appending _2, _3, etc.
	name_ver := make(map[string]int)
	id_to_name := make(map[string]string, len(keep_ids))
	for _, id := range keep_ids {
		name := flat[id].requested_name
		if name != "" && name_count[name] > 1 {
			name_ver[name]++
			if name_ver[name] > 1 {
				name = fmt.Sprintf("%s_%d", name, name_ver[name])
			}
		}
		id_to_name[id] = name
	}

	// Build ResolvedTypes with sentinel-wrapped ref strings.
	result := make(ResolvedTSTypes, len(keep_ids))
	for _, id := range keep_ids {
		entry := flat[id]
		result[ID(id)] = ResolvedTSType{
			ID:   ID(id),
			Name: id_to_name[id],
			Body: node_to_ts(entry.node),
		}
	}

	// Replace all sentinel-wrapped IDs with resolved names.
	for id, td := range result {
		for replaced_id, name := range id_to_name {
			if name != "" && strings.Contains(td.Body, replaced_id) {
				td.Body = strings.ReplaceAll(td.Body, replaced_id, name)
			}
		}
		result[id] = td
	}

	return result
}

/////////////////////////////////////////////////////////////////////
/////// Ref collection
/////////////////////////////////////////////////////////////////////

func collect_all_ref_ids(entries map[string]*walk_entry) map[string]bool {
	refs := make(map[string]bool)
	all_ids := make([]string, 0, len(entries))
	for id := range entries {
		all_ids = append(all_ids, id)
	}
	for _, entry := range entries {
		collect_refs(entry.node, refs, all_ids)
	}
	return refs
}

func collect_refs(node *type_node, refs map[string]bool, known_ids []string) {
	if node == nil {
		return
	}
	switch node.kind {
	case kind_ref:
		refs[node.ref_id] = true
	case kind_raw:
		for _, id := range known_ids {
			if strings.Contains(node.raw_ts, id) {
				refs[id] = true
			}
		}
	case kind_object:
		for _, f := range node.fields {
			collect_refs(f.node, refs, known_ids)
		}
	case kind_array:
		collect_refs(node.elem, refs, known_ids)
	case kind_map:
		collect_refs(node.key_type, refs, known_ids)
		collect_refs(node.val_type, refs, known_ids)
	}
}

/////////////////////////////////////////////////////////////////////
/////// TS string generation from type_nodes
/////////////////////////////////////////////////////////////////////

func node_to_ts(node *type_node) string {
	if node == nil {
		return "null"
	}

	switch node.kind {
	case kind_null:
		return "null"
	case kind_unknown:
		return "unknown"
	case kind_bool:
		return "boolean"
	case kind_number:
		return "number"
	case kind_string:
		return "string"
	case kind_raw:
		return node.raw_ts
	case kind_ref:
		return node.ref_id // sentinel-wrapped, resolved later
	case kind_object:
		return object_to_ts(node.fields)
	case kind_array:
		return fmt.Sprintf("Array<%s>", node_to_ts(node.elem))
	case kind_map:
		return fmt.Sprintf(
			"Record<%s, %s>",
			node_to_ts(node.key_type),
			node_to_ts(node.val_type),
		)
	default:
		return "unknown"
	}
}

func object_to_ts(fields []field_node) string {
	if len(fields) == 0 {
		return "Record<never, never>"
	}

	var sb strings.Builder
	sb.WriteString("{\n")
	for _, f := range fields {
		sb.WriteString("\t")
		sb.WriteString(ts_property_name(f.name))
		if f.optional {
			sb.WriteString("?")
		}
		sb.WriteString(": ")
		sb.WriteString(node_to_ts(f.node))
		sb.WriteString(";\n")
	}
	sb.WriteString("}")
	return sb.String()
}

func ts_property_name(name string) string {
	if name == "" {
		return strconv.Quote(name)
	}
	for i, r := range name {
		if i == 0 {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
				r == '_' || r == '$' {
				continue
			}
			return strconv.Quote(name)
		}
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9') || r == '_' || r == '$' {
			continue
		}
		return strconv.Quote(name)
	}
	return name
}
