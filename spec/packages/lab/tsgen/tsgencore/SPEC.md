# lab/tsgen/tsgencore Specification

Status: Active  
Last Updated: 2026-02-09  
Owner: `lab/tsgen/tsgencore`

## Scope

Package-owned type-graph mining and TypeScript-type synthesis contracts in
`lab/tsgen/tsgencore/**`.

Current evidence note:

- Requirements are mined from `lab/tsgen/tsgencore/tsgencore.go` and
  `lab/tsgen/tsgencore/tsgencore_test.go`.
- Integration assertions in `lab/tsgen/generate_ts_content_test.go` provide
  additional corroboration for override/embedding interactions.

## Requirements

- `LAB-TSGEN-TSGENCORE-001` Type-processing entrypoint contract. `ProcessTypes`
  MUST traverse each provided ad-hoc type and merge all results into a single
  `Results` payload.
- `LAB-TSGEN-TSGENCORE-002` Nil-input traversal contract. Nil ad-hoc entries or
  nil type instances MUST produce empty traversal output without panicking.
- `LAB-TSGEN-TSGENCORE-003` Type lookup fallback contract. `Results.GetTypeInfo`
  MUST return merged type info when present; otherwise it MUST synthesize
  fallback info from effective reflect type + basic TS type.
- `LAB-TSGEN-TSGENCORE-004` Merge inclusion/filter contract. Merged output MUST
  include only types that are roots, referenced, or required by internal ID
  references discovered inside emitted TS strings.
- `LAB-TSGEN-TSGENCORE-005` Deterministic merge-order and name-collision
  contract. Final type IDs MUST be sorted deterministically; duplicate requested
  names MUST receive stable numeric suffixes (`_2`, `_3`, ...).
- `LAB-TSGEN-TSGENCORE-006` Internal-ID rewrite contract. Internal
  `$tsgen$...$tsgen$` references in emitted TS strings MUST be replaced with
  resolved names; unresolved IDs MUST fall back to `unknown` with warning
  output.
- `LAB-TSGEN-TSGENCORE-007` Struct-field selection contract. Struct generation
  MUST skip unexported fields and JSON-omitted fields, and use JSON field names
  (including renamed fields) for emitted keys.
- `LAB-TSGEN-TSGENCORE-008` Embedded-field flattening contract. Untagged
  anonymous embedded structs MUST flatten in depth-first source order; tagged
  embedded fields MUST remain nested references.
- `LAB-TSGEN-TSGENCORE-009` Type-override precedence contract. Field type
  resolution MUST apply precedence: `TSType()` method override > `ts_type`
  struct tag > reflection-derived type.
- `LAB-TSGEN-TSGENCORE-010` Optionality contract. Pointer fields and `json` tags
  containing `omitempty` or `omitzero` MUST emit optional (`?`) TypeScript
  fields.
- `LAB-TSGEN-TSGENCORE-011` Go-to-TS mapping contract. Core mappings MUST
  include: interface->`unknown`, bool->`boolean`, numeric scalars->`number`,
  string->`string`, `[]byte`->`string`, slices/arrays->`Array<T>`,
  maps->`Record<K,V>`, `time.Time`->`string`, and `time.Duration`->`number`.
- `LAB-TSGEN-TSGENCORE-012` Empty-object contract. Structs with no emitted
  fields MUST map to `Record<never, never>`.
- `LAB-TSGEN-TSGENCORE-013` TSTyper interface-discovery contract. `getTSTypeMap`
  MUST support both value-receiver and pointer-receiver `TSType` implementations
  and initialize embedded nil pointers when invoking these methods.
- `LAB-TSGEN-TSGENCORE-014` Identifier/ID normalization contract. Type names
  MUST be sanitized to JS identifier-safe names and internal IDs MUST encode
  requested-name disambiguation when requested and natural names differ.
