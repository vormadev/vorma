# lab/tsgen Specification

Status: Active  
Last Updated: 2026-02-09  
Owner: `lab/tsgen`

## Scope

Package-owned TypeScript generation contracts in `lab/tsgen/**`, excluding
`lab/tsgen/tsgencore/**` owner semantics.

Current evidence note:

- Requirements are mined from `lab/tsgen/generate_ts_content.go`,
  `lab/tsgen/to_file.go`, `lab/tsgen/statements.go`, and
  `lab/tsgen/generate_ts_content_test.go`.
- `lab/tsgen/tsgencore/**` behavior is owned by `LAB-TSGEN-TSGENCORE-*`
  requirements.

## Requirements

- `LAB-TSGEN-001` Type-merge input contract. `GenerateTSContent(opts)` MUST
  build merged type results via `optsToMerged`, including `opts.AdHocTypes` and
  collection phantom types except phantom values implementing `TSTyperRaw`.
- `LAB-TSGEN-002` Output section-order contract. `GenerateTSContent` MUST emit
  sections in this order: generated-file banner, optional `Collection` block,
  optional `Ad Hoc Types` block, optional `Extra TS Code` block.
- `LAB-TSGEN-003` Collection declaration contract. When `opts.Collection` is
  non-empty, collection output MUST be emitted as a `const` array with
  `as const`; `CollectionVarName` defaults to `tsgenCollection`;
  `ExportCollectionArray=true` prefixes `export`.
- `LAB-TSGEN-004` Arbitrary-property serialization contract. Collection
  arbitrary properties MUST be emitted as JSON-compatible JS values using
  `formatJSValue`, and marshal failures MUST be returned as errors.
- `LAB-TSGEN-005` Phantom-property rendering contract. Collection phantom
  properties MUST render as typed `null` markers, using `TSTyperRaw` when
  present and merged type-info fallback otherwise; `null` and
  `Record<never, never>` phantom outputs are omitted.
- `LAB-TSGEN-006` Deterministic ordering contract. Collection property lines
  MUST be sorted per item; rendered collection items MUST be sorted; exported
  type lines MUST be sorted.
- `LAB-TSGEN-007` Type-export emission contract. `getExports` MUST emit
  `export type <ResolvedName> = <TSStr>;` only for merged types with non-empty
  `ResolvedName`.
- `LAB-TSGEN-008` Extra TS passthrough contract. `ExtraTSCode` MUST be trimmed
  and appended verbatim (under the extra-code section comment) when non-empty.
- `LAB-TSGEN-009` Generate-content error contract. `GenerateTSContent` MUST
  return collection-rendering errors and otherwise return generated content
  without mutating filesystem state.
- `LAB-TSGEN-010` File output contract. `GenerateTSToFile` MUST require
  non-empty `OutPath`, call `fsutil.EnsureDir(filepath.Dir(OutPath))`, then
  write generated content to `OutPath` using `os.WriteFile(..., os.ModePerm)`.
- `LAB-TSGEN-011` File output error-message contract. `GenerateTSToFile` MUST
  return: `outpath is required`, `failed to ensure out dest dir: ...`, or
  `failed to write ts file: ...` for respective failure modes.
- `LAB-TSGEN-012` Statement-builder contract. `Statements.Raw`,
  `Statements.Serialize`, and `Statements.Enum` MUST append statement tuples;
  `BuildString` MUST render each tuple as `<prefix> = <value>;\n`.
- `LAB-TSGEN-013` Value serialization contract. `serialize(any)` MUST use
  `json.MarshalIndent` and panic on marshal failure; non-string/non-int/non-bool
  values MUST append ` as const`.
- `LAB-TSGEN-014` Union-helper contract. `StringUnion` and `TypeUnion` MUST
  return `""` for empty input, otherwise join members using `" | "`
  (`StringUnion` uses single-quoted string literals).
