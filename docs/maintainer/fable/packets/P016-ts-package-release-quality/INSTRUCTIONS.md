# P016 — TS package release-quality pass (jsdoc + thermo-nuclear findings)

Read `docs/maintainer/fable/README.md` (the packet protocol binds you), then
`LEARNINGS.md`, `STATE.md`, repo-root `AGENTS.md` (the documentation strategy: jsdoc IS
the user documentation for the TS surface, same standing as rust-doc),
`docs/maintainer/REMINDERS.md`, and the review bar:
`docs/maintainer/skills/thermo-nuclear-system-review/SKILL.md`. Model packets: P011-P015
(same shape, Rust side). Inputs: the census TS-surface sections and member-level sweep in
`docs/maintainer/tickets/board-api-coverage/PRESSURE_TEST_CENSUS.md` (including finding
F-7), `api_inventory_ts.txt` in the same directory, and board's client code as the
canonical consumer.

## Context

The Rust crates are documentation-complete; this packet is the TypeScript half:
`packages/vorma` (core, kit modules, the react/preact/solid adapters, vite plugin) and
`packages/create-vorma`. Census F-7 is the charter: the integration-pattern surfaces
(`workIndicator`'s nprogress-shaped contract, `apiClient.toIdentityArray`'s react-query
purpose, `useRouteSync`, the typed-args wrapper idiom from F-10, prefetch cancellation
semantics) taught nothing by name alone — the jsdoc must lead with when/why/how, at the
same teaching bar as the Rust side, with board's real usage as the reference examples.

## Scope

1. **jsdoc sweep.** Every exported item across the public entry points: types, functions,
   hooks, component props (Link's full prop breadth per the census member sweep), options
   bags (ClientOptions members incl. `workIndicator`/`revalidateOnWindowFocus`/
   `useViewTransitions`/`onRouteUpdate`/`linkDefaultProps`), the apiClient family
   (`query`/`queryOrThrow`/`mutate`/`mutateOrThrow`/`toIdentityArray`) with the
   auto-revalidation default stated LOUDLY (census F-8: manual revalidate-after-mutate is
   the documented anti-pattern), the read models (`RouteState`/`WorkState`), kit modules
   (generic, backend-agnostic framing per REMINDERS), and the vite plugin's options.
   Adapters: write once at the shared/core layer where the code is shared; adapter files
   document only genuine divergences (DRY applies to docs).
2. **Enforcement mechanism.** Rust has `deny(missing_docs)`; TS needs an equivalent so
   coverage cannot rot. Investigate and PROPOSE with trade-offs, then land the best fit:
   candidates include an oxlint jsdoc rule if the pinned oxlint supports one adequately, a
   `typedoc`-based coverage check, or a minimal checked-in script over the exported
   surface (minimal-tooling rule applies — no Rube Goldberg). A new devDependency requires
   escalation-in-report with the justification. If NO sane mechanism exists, that
   conclusion with evidence is an acceptable outcome — but the default assumption is one
   exists.
3. **Generated-types docs check.** The documentation strategy says generated docs derive
   from source comments. Verify whether `TsGen` carries Rust doc comments through to the
   generated `vorma.gen.ts` declarations. If it does not, that is a FINDING with a ticket
   (the fix belongs to vorma-contract's generator, not this packet) — the generated file
   is many users' first contact with their own types.
4. **Thermo-nuclear review, FINDINGS MODE**, over the package source (core/kit/adapters/
   vite): direct-landable class as before (docs, coverage-strengthening tests,
   zero-behavior private cleanups); structural/public-surface findings analyzed and
   reported for the batch triage. The `__internal`/`__`-prefixed surface is exempt by
   standing census ruling (the prefix IS the stance) — do not document it as public.
5. **Checklist verdict** with evidence.

## Hard constraints

- No public API changes; no new runtime dependencies (devDependency for enforcement only
  via escalation-in-report); kit stays generic (never Vorma-backend-aware — REMINDERS).
- vorma.gen.ts is generated — never hand-edit (generator findings go to scope item 3).
- Tests never cheat; vitest additions must be deterministic.
- Scoped fmt writes only (formatter hazards in ticket
  oxfmt-markdown-corruption-and-nonconvergence); no git actions; no network installs
  (node_modules is current — if the enforcement mechanism needs a package not present,
  that is an escalation with the exact package and why, not an install).
- Unexpectedly dirty unowned files are escalations, never cleanup.

## Definition of done

- Every exported public item documented at the teaching bar; the enforcement mechanism
  landed (or the no-sane-mechanism conclusion evidenced); generated-types docs verdict
  delivered (finding+ticket if negative).
- Gates green: `make ts-typecheck` (all projects), ts-lint, vitest 851+ (any additions
  deterministic), scoped oxfmt checks, plus the new enforcement check itself green;
  workspace Rust gates untouched-green (this packet should touch no Rust).
- REPORT.md per template — put the ENTIRE report in your final message body (the
  report-file write guardrail fires for every executor; only the final message reaches the
  orchestrator).
