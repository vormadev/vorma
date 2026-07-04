# P016 Report

Provenance note: delivered by the executor (Sonnet 5 subagent, 2026-07-02) in its final
message per the anti-truncation convention; Fable placed it (transport escaping undone).

## What changed

Core (20 files): jsdoc to the teaching bar across the api-client family
(`resource_types.ts` — F-7/F-8's home, auto-revalidation stated loudly),
`work_indicator.ts` (F-7's exemplar), `route_types.ts`/`work_state.ts` (read models),
`ui_adapter_types.ts` (`VormaClient`, the largest teaching block), `client_core_types.ts`,
`client_wasm/matcher.ts`, `constants.ts` (bare re-exports converted to documented local
re-declarations — a bare `export {X} from` cannot carry a doc comment the generated source
lacks, verified), and the remaining core modules.

Adapters (3): `react.tsx` carries the canonical `createVormaClient` teaching doc
(mirroring board's real setup); preact/solid document divergences only — including the
initially-missed real one: Preact's hooks return `ReadonlySignal<T>` ("signal" mode),
caught by a full-file diff after a shallow symbol diff wrongly suggested near-identity;
Solid's accessor mode verified against the real mount in the framework tests.

Vite plugin: module doc + the sole export documented with board's real config usage.

Kit (9 files, backend-agnostic verified): all modules to the bar, including all 20
converters (initially left terse; the new enforcement checker flagged them and the strict
gate was kept over a special-case exemption), a spec-accuracy fix to the csrf
`__Host-`/`__Dev-` explanation, and the theme module's plain comment converted to real
jsdoc with claims verified.

`create-vorma/main.ts`: an honest STALE marker (zero exports, Go-era bin script, ruled
out-of-scope by the standing `create-vorma-rewrite` ticket) instead of polishing doomed
code.

New: `check_jsdoc_coverage.ts` (repo root, the enforcement gate) + root `tsconfig.json`

- Makefile wiring (`ts-jsdoc-coverage` in `ts-gate`; the tsconfig in `ts-typecheck`).

New tickets: `ts-package-release-quality-findings` (3 findings, held for batch triage);
`tsgen-doc-comments-not-carried-to-generated-ts` (scope item 3's negative verdict).

No public API changes; no new runtime dependencies; zero Rust files touched (verified by
diff-content inspection against P015's pre-existing uncommitted work).

## Decisions made

1. Enforcement = a checked-in TS-compiler-API script, not oxlint's jsdoc plugin — probed
   first: oxlint's jsdoc rules validate CONTENTS but have no require-jsdoc equivalent
   (confirmed against its rule schema and an undocumented-exports fixture); typedoc
   absent. The script uses the already-present `typescript` package via Node's native
   strip-types: zero new dependencies, no build step.
2. Enforcement scope = the 14 real tsdown entry points (kept DRY with the actual build
   config), `vorma/__internal` exempt per the census's own prefix-IS-the-stance ruling;
   internal-only reachability traced with the compiler API, not guessed.
3. The checker's own first result was distrusted and a resolution bug fixed before
   trusting it: TypeScript's self-reference resolution hit a stale prebuilt
   `.dist/core/_index.d.ts` producing 149 false positives — fixed with a source-anchored
   paths map so the check can never read stale output.
4. converters.ts judgment call reversed when the checker flagged it (see above).
5. Generated-types check: a three-layer trace (derive attribute allowlist; the
   Type/TypeDef/FieldDef model has no doc field; the renderer's single hand-templated
   comment) plus empirical confirmation against board's generated file. Negative; ticketed
   against the generator.
6. ~40 `any` usages sampled and traced; cleared with reasoning except the one genuine
   inconsistency (finding 2).

## Doc-sweep stats

176/176 public exports documented across the 13 non-exempt, non-empty entry points
(react/preact/solid 44 each; vite 1; kit modules 43 total across 9; create-vorma 0/0 by
nature). `vorma/__internal` informational: 85/92, the 7 undocumented being exactly the
internal-only surface.

## Enforcement mechanism

`check_jsdoc_coverage.ts` → `make ts-jsdoc-coverage` → in `ts-gate`. Resolves exports past
re-export aliases to real declarations (docs live once, at the shared definition);
`__`-prefix and `*.gen.ts` declarations are principled exemptions (gen-file hits reported
as ACKNOWLEDGED, never silently skipped; currently zero). Proven non-vacuous by an
intentional break-then-restore. No escalation needed (no new devDependency).

## Generated-types docs verdict

NEGATIVE — TsGen does not carry Rust doc comments into `vorma.gen.ts`. Ticket:
`tsgen-doc-comments-not-carried-to-generated-ts` (belongs to vorma-contract's generator).

## Findings

1. `_index.ts` silently drops eight items from `create_client_core.ts`'s own
   explicitly-labeled Public Types/Constants section — dead public-looking surface one hop
   from published; two of the eight proven load-bearing via direct source imports in
   tests.
2. Two structurally identical dev-time cross-bundle globals typed inconsistently: the HMR
   hook via proper `declare global`; the `Symbol.for("vorma-data-revalidate-fn")` channel
   via two `as any` casts with no declaration anywhere.
3. `create_client_core.ts`'s ~1565-line function body — watch-only with reasoning (the
   file already decomposes five other concerns correctly; further extraction is marginal
   deps-threading).
4. (Standalone ticket) TsGen doc-comment carriage, above.

## Checklist verdict

13/13 PASS; findings 1-2 attached to Nothing-weird/No-cruft, finding 3 to
Nothing-overly-complicated. No FAILs.

## Gate results

`make ts-typecheck` (all 9 projects incl. the new tsconfig) green; full-repo oxlint green;
full-repo oxfmt check green (317 files); the new jsdoc gate green (176/176, break-proven);
vitest 851/851; Rust gates untouched-green (zero Rust modifications; the one `.wasm`
byte-diff is the known accepted non-deterministic-build side effect of running ts-build,
already documented in STATE.md).

## Escalations / open questions

None requiring the maintainer pre-landing. Findings held per the sibling-packet pattern.

## Discovered out-of-scope work

The two new tickets; nothing else. Pre-existing dirty tree (P015's uncommitted work)
untouched throughout, verified at multiple checkpoints.
