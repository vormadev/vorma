# P017 Review

Reviewer: Fable (orchestrator). Date: 2026-07-02. Executor: Sonnet 5 subagent.

## Verdict

**Accepted. Packet closed.** The audit was performed the way it must be: every claim
checked against current source with file:line citations, and the document trusted NOWHERE
— including the one place where the audit's reference material was itself wrong (the
`VitePluginConfig` case, where ARCHITECTURE.md turned out to be more accurate than the
crate's own doc comment). The headline catch is real drift with teeth: the "Known
asymmetries" section claimed watch-root config changes are "guarded by the explicit
config-transition error," and no such mechanism exists — the doc was promising users a
guard the code never had. The correction states the honest behavior (silently ineffective
until restart) and contrasts it with the ServerBuildTarget restart error, which IS real.

## Rulings on the escalated judgment calls (Fable)

1. **P019 shared-type dedup: ADD (overruled, executor's proposed shape).** The rule
   changes what a duplicate type name MEANS at the public app API surface — that is
   map-level, not manual-level. Landed by Fable as one clause on the `vorma-contract`
   bullet, per the executor's own proposed text.
2. **Board maintenance-worker pattern: DO NOT ADD (upheld).** ARCHITECTURE.md has zero
   example-app content by design; the framework-level fact rides the new `vorma::tasks`
   re-export bullet the executor added. Correct instinct.
3. **`vorma-bench`: DO NOT ADD (upheld).** Consistent with the document's shipped-
   architecture framing.

## Fable actions on the discovered items

- **Watch-root gap (no ticket existed):** an addendum recorded on
  `dev-watch-excludes-not-pruned-from-os-watch` (the sibling watch-plan-vs-OS-watch
  divergence ticket) rather than a new ticket — same subsystem, same eventual fix locus
  (reconciling the OS watch set against the swapped plan).
- **`vite_plugin_contract.rs:30-34` doc-comment fix:** sanctioned as a Fable direct
  landing (one-line rustdoc, zero behavior), QUEUED until P018 completes — it is a source
  file in vorma-build and P018 is concurrently packaging that crate; changing files
  mid-packaging-run would trip P018's dirty-tree tripwires.

## Findings

No executor issues found. The verification-of-verifier discipline (re-checking its own
sub-agent's citations before acting on them) is the protocol working as intended.

## Consequence

P017 closes. ARCHITECTURE.md is claim-by-claim accurate against shipped code as of today.
Remaining in Phase D: P018 (in flight), the queued one-line rustdoc fix, then the batched
surface-polish triage to the maintainer.
