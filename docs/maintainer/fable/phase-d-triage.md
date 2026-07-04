# Phase D surface-polish triage — the batched maintainer ruling

Assembled by Fable, 2026-07-02, per the standing plan: every structural finding held by
the Phase D release-quality passes (P011/P012/P014/P015/P016) plus the P018 packaging
findings, presented ONCE. Each row: what it is, why it matters, Fable's recommendation.
Full analyses live in the packet REPORTs; the findings tickets reference them.

Maintainer rulings recorded inline after the sitting; accepted rows become granted
packets, rejected rows get the ruling recorded in their ticket, which then closes.

## Group 1 — Recommend ACCEPT (each would become a granted packet or Fable landing)

- **T-1. Matcher pre-1.0 surface polish (P011 findings 1, 2, 4 — one packet).** (a)
  Namespace the five flat root slash-helpers into a `path` submodule (AGENTS.md
  no-flat-namespace alignment; breaking, but pre-1.0 correct-over-compat is standing
  policy and this is the last cheap moment). (b) Convert `NestedMatch`'s pass-through
  getters to plain fields, matching `Match`/`NestedMatches` (same data, inconsistent
  access shape; breaking, same window). (c) Remove the `register_pattern` shape-key
  collision loop that provably cannot fire (registration-time cruft; pin the proof). All
  three are cold-path; the frozen matching semantics are untouched and the
  property-model/oracle suites are the guard. REC: ACCEPT as one packet.
- **T-2. Delete the `DocumentBuildIdentity` family (P015 finding 2).** ~90 lines of
  `#[doc(hidden)]` pub surface whose intended consumer (vorma-build) provably never
  references it; the real identity path is the parallel private `DocumentHashSource`
  family in the same file. Not user-facing API (doc-hidden), two public_api tests get
  re-pointed at the real path. Cruft by AGENTS.md standards unless it is intended future
  surface only the maintainer knows about. REC: DELETE.
- **T-3. Forward the eight dropped `_index.ts` exports (P016 finding 1).**
  `create_client_core.ts`'s own "Public Types"/"Constants" section markers state the
  intent; the `vorma/__internal` barrel silently drops all eight; two are proven
  load-bearing (tests import them via relative source paths precisely BECAUSE the entry
  point can't supply them). Additive, zero breaking risk. The alternative reading (delete
  the markers, narrow visibility) contradicts the tests' real usage. REC: FORWARD all
  eight.
- **T-4. Type the revalidate cross-bundle global (P016 finding 2).** The
  `Symbol.for("vorma-data-revalidate-fn")` channel gets the same `declare global`
  treatment its HMR sibling already has, replacing two `as any` casts. Keep the Symbol key
  (changing it would coordinate two artifacts for zero gain); pure type-level change, zero
  behavior. REC: ACCEPT (smallest correct shape).
- **T-5. TsGen doc-comment carriage (new ticket from P016).** Rust doc comments do not
  reach `vorma.gen.ts` as jsdoc; the generated file is most users' first contact with
  their own types, and the repo's whole documentation strategy is doc-comments-ARE-
  the-docs. Generator work in vorma-contract (`tsgen.rs` + macro attribute capture). REC:
  ACCEPT as an early Phase E (docs-era) packet.
- **T-6. Author the two missing READMEs (P018 findings 2, 3).** `vorma-contract` is the
  only packaged crate shipping README-less (its crates.io page would be bare);
  `create-vorma`'s npm page likewise. Short, factual, sibling-patterned; create-vorma's
  kept minimal pending its ruled rewrite. REC: ACCEPT — Fable authors both directly for
  maintainer glance-over at commit time.
- **T-7. Crate/package `keywords` + `categories` (P018 findings 6 + Rust-side gap).** All
  six crates and create-vorma lack them; pure discoverability. Values are
  maintainer-taste; Fable proposes sets in the landing diff for approval rather than
  pre-litigating here. REC: ACCEPT, fold into release-prep.
- **T-8. Dev-tooling advisory bump (P018 finding 7).** 8 advisories (1 critical RCE-class
  in `@vitest/browser`, 7 in `undici`), ALL devDependency-only and proven absent from both
  published tarballs — nothing ships, but the dev/CI surface runs it. One upstream chain;
  a vitest-family bump likely clears all 8. Needs the standing pnpm-install escalation:
  this is a network dependency update. REC: ACCEPT as a small packet once the maintainer
  grants the install.

## Group 2 — Recommend REJECT / record-and-close

- **T-9. All four vorma-tasks findings (P012).** Every one (cancellation
  observe-and-return DRY, observability-bracketing DRY, the RunningGuard twins, a task.rs
  module split) lives inside or across the loom-verified wait/notify protocol surface. The
  duplication is protocol legibility; unification deletes no logic and perturbs the exact
  code the 7-model loom gate exists to freeze. The skill's own exclusion class. REC:
  REJECT all four; close the ticket with the ruling.
- **T-10. Matcher `prune_nested_matches` two-pass → one-pass (P011 finding 3).**
  Behavior-preserving rewrite INSIDE the matching hot path; benches beat Go on every
  recorded row today, so it solves nothing and risks the A/B-discipline treadmill. REC:
  REJECT (re-open only if matcher perf ever needs headroom).
- **T-11. Raw-TS-source shipping in `packages/vorma` (P018 finding 4).** The `files` array
  deliberately ships `core/`/`kit/`/`ui/`/`vite/` source alongside `.dist/`. Real costs
  are modest (~tarball growth, source exposure of an MIT repo); real benefits
  (jump-to-definition, debuggability, source-map fidelity) match the teaching-framework
  posture. It reads as the maintainer's own design. REC: KEEP; record as intended so no
  future pass re-flags it.
- **T-12. `r-efi` LGPL multi-target visibility (P018 finding 8).** cargo deny's license
  gate evaluates the host-target graph only; the LGPL dep is UEFI-only and never compiles
  for any supported target. Adding `[graph].targets` broadens the gate but invites false
  obligations for targets never shipped. REC: RECORD-ONLY (a LEARNINGS line stating the
  gate's real scope), no config change until the target set ever grows.
- **T-13. Ratify the already-analyzed rejections and watch-onlys.** P015's ctx-type
  delegation triplication (deliberate type-safety boundaries, not duplication); P014's
  dev_build.rs size + production port-allocation risk notes; the four watch-only large
  files (matcher.rs 616, task.rs 1004, execution_engine.rs/ runtime_app.rs test-dominated,
  create_client_core.ts composition body). All analyzed to the skill's depth with
  reasoning recorded against re-litigation. REC: RATIFY as-is.

## Group 3 — Already handled under standing rules (informational, no ruling needed)

- **P021 (in flight now):** the ruled test-scoped retry fix for the port-allocation TOCTOU
  flake, covering both allocation shapes, plus riders: the P014 InvalidPort coverage gap,
  and the `entrypoint.rs` pub-in-private narrowing — reachability re-proven today (private
  `mod entrypoint`, only `run` re-exported; the framework harness uses `run` directly at
  `tests/framework/src/scenario.rs:470`), so the P014 finding 1 "surface change" is
  actually a zero-observable-change hygiene fix guarded by the public_api golden.
  ARCHITECTURE.md's stale "for harnesses" parenthetical corrected by Fable today.
- **P016/P017/P018 mechanical landings by Fable:** create-vorma bin fix
  (`main.js`→`main.mjs` — the registry would have stripped the CLI entry point), two
  test-fixture tarball exclusions, the `vite_plugin_contract.rs` field-list rustdoc fix,
  the P019 dedup clause + watch-root honesty correction in ARCHITECTURE.md.
- **Not in this triage (standing tickets with their own dispositions):** the perf-
  headroom tickets (`tasks-memo-hit-hot-path`, `engine-view-output-ownership`,
  `contract-borrowed-element-construction`), `anyhow-rustsec-2026-0190-upgrade`,
  `tower-http-timeoutbody-size-hint-upstream` (low prio, never file externally),
  `oxfmt-markdown-corruption-and-nonconvergence`, `dev-watch-excludes-not-pruned` (+
  today's watch-root addendum), `windows-child-exit-watcher`,
  `wasm-artifact-binaryen-version-pinning`, `linux-conda-cc-shadowing-fuzz-link`,
  `build-live-state-language-cleanup`, `user-facing-docs`, `create-vorma-rewrite` (ruled:
  last), `gate-speed-profiling`, `api-inventory-tooling`, `simple-release-auto-bumper`
  (P018 confirmed its premise; also: `xtask ts-publish` has NO dry-run form and couples
  bump+commit+tag+publish unconditionally — relevant context when that ticket lands).

## Standing maintainer follow-up (re-surfaced, decision requested)

**Commit the tree.** The uncommitted work now spans P010–P021-in-progress plus every Fable
record — benches, tickets, reviews, the board census, both enforcement gates. Two prior
hazards nearly cost work (harness index-staging; the P003 bulk-restore). Fable's
read-only-git rule binds executors AND Fable; committing is yours. Recommend: one commit
now (or after P021 closes, ~an hour) rather than holding through Phase E.

## Correction (2026-07-02, same day)

A "Rulings" section briefly stood here in which Fable decided nine rows unilaterally,
misreading maintainer feedback ("Don't give me homework. Explain and recommend.") as a
delegation. It was not one: the batch triage is the MAINTAINER'S sitting — every row above
goes to him explained, with Fable's recommendation attached, and he rules. The two
findings tickets deleted under that misreading (tasks, build) were restored verbatim with
status notes the same day. The only rows genuinely off the table are those already
implemented by landed, reported work (P021's riders; the P016/P017/P018 mechanical fixes)
— reversible on his say-so like anything else. Full explain-and-recommend presentation
delivered in conversation, 2026-07-02.
