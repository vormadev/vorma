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

## Scenario Catalog

- `LAB-TSGEN-SCN-001` (covers `LAB-TSGEN-001`): Merge-input fixtures MUST verify
  ad-hoc and collection phantom inputs are merged while `TSTyperRaw` phantom
  entries are excluded from merged export processing.
- `LAB-TSGEN-SCN-002` (covers `LAB-TSGEN-002`): Generated output fixtures MUST
  verify banner/comment section ordering for collection, ad-hoc type exports,
  and extra TS blocks.
- `LAB-TSGEN-SCN-003` (covers `LAB-TSGEN-003`): Collection-output fixtures MUST
  verify const-array emission, default collection variable naming, and optional
  `export` prefix behavior.
- `LAB-TSGEN-SCN-004` (covers `LAB-TSGEN-004`): Arbitrary-property fixtures MUST
  verify JSON-compatible literal emission and marshal-error propagation.
- `LAB-TSGEN-SCN-005` (covers `LAB-TSGEN-005`): Phantom-output fixtures MUST
  verify `TSTyperRaw` handling, merged type-info fallback, and omission of null
  / `Record<never, never>` phantom outputs.
- `LAB-TSGEN-SCN-006` (covers `LAB-TSGEN-006`): Determinism fixtures MUST verify
  sorted property lines, sorted collection items, and sorted export lines.
- `LAB-TSGEN-SCN-007` (covers `LAB-TSGEN-007`): Export fixtures MUST verify
  `getExports` includes only merged types with non-empty resolved names.
- `LAB-TSGEN-SCN-008` (covers `LAB-TSGEN-008`): Extra-code fixtures MUST verify
  trimmed `ExtraTSCode` passthrough under the extra-code comment block.
- `LAB-TSGEN-SCN-009` (covers `LAB-TSGEN-009`): Error-path fixtures MUST verify
  `GenerateTSContent` propagates collection-rendering errors and has no
  filesystem side effects.
- `LAB-TSGEN-SCN-010` (covers `LAB-TSGEN-010`): File-output fixtures MUST verify
  `OutPath` validation, ensure-dir call ordering, and write-to-path behavior.
- `LAB-TSGEN-SCN-011` (covers `LAB-TSGEN-011`): File-output error fixtures MUST
  verify exact wrapped error prefixes for missing outpath, ensure-dir failure,
  and write failure.
- `LAB-TSGEN-SCN-012` (covers `LAB-TSGEN-012`): Statement-builder fixtures MUST
  verify append semantics and `<prefix> = <value>;\n` rendering shape.
- `LAB-TSGEN-SCN-013` (covers `LAB-TSGEN-013`): Serialization fixtures MUST
  verify `json.MarshalIndent` formatting, panic-on-marshal-failure behavior, and
  ` as const` suffix rules.
- `LAB-TSGEN-SCN-014` (covers `LAB-TSGEN-014`): Union-helper fixtures MUST
  verify empty-input empty-string returns and pipe-delimited union joining.
