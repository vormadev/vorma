# Roadmap to Release

Sequenced plan, owned by Fable. Executors take packets from `packets/` in order unless
the maintainer redirects. A packet is done when its `REPORT.md` exists, its definition
of done is met, and Fable's `REVIEW.md` says accepted.

Status legend: `[ ]` not started · `[>]` in progress · `[R]` awaiting review · `[x]`
accepted.

## Phase A — Stabilize the baseline

Goal: every gate green, the tasks-crate performance bar restored, the recorded state
honest. Nothing else proceeds on a red or dishonest baseline.

- `[ ]` **P001 — Green all gates and record the baseline.** Fix the one clippy error,
  run every gate (rust, loom, TS, e2e smoke), record results in STATE.md.
- `[ ]` **P002 — Tasks perf restoration.** Replace blake3 hot-path fingerprinting with
  keyed SipHash, restore the poll-once fast path, attribute and reduce ParallelBatch
  per-sibling overhead, harden the fingerprint hasher, re-measure honestly against the
  STATE.md tables. Bar: beat Go on every row except the two with accepted structural
  explanations.

## Phase B — Finish the standing reviews

- `[ ]` **P003 — Board 100% API coverage audit.** Execute the standing policy
  (`AGENTS.md`, `docs/maintainer/board-example/README.md`) against the full public API
  inventory; absorb the coverage orphaned by the notes-example deletion (extended-cache
  task in app context, request-level suite patterns). Consumes ticket
  `board-api-coverage` and its census.
- `[ ]` **P004 — Request-path performance review.** The vorma engine review: measure
  from the moment a request becomes owned by vorma to the moment vorma returns its
  result (adapter excluded), find matcher-review-class costs, fix within frozen
  semantics, add `bench-engine` recording. Consumes ticket
  `router-request-path-review`. Fable writes the analysis addendum before execution
  starts.

## Phase C — Board completion

Remaining board census features and production build. Packets to be authored by Fable
when Phase B closes (inputs: the census under `tickets/board-api-coverage/`, P003's
report). Expected shape: one packet per census feature cluster, then a prod-build
packet.

## Phase D — Release-quality sweeps

To be authored as Phase C closes:

- Doc comments for every public API, per the documentation strategy in `AGENTS.md`
  (rust-doc and jsdoc are the primary user documentation). One packet per crate, plus
  one for the TS package.
- `ARCHITECTURE.md` accuracy pass against the shipped code.
- Per-crate compliance pass against
  `docs/maintainer/skills/thermo-nuclear-system-review/SKILL.md`.
- Packaging dry-runs (`make rust-package`, TS publish dry-run) and dependency policy
  review (`cargo deny`, supply-chain inventory).

## Phase E — Endgame

- create-vorma rewrite (maintainer ruling: this comes last).
- vorma.dev docs site per the strategy in `AGENTS.md` (a Vorma app embedding the
  generated API reference and the Board example).
- Release.

## Standing inputs

- `STATE.md` — current gates, numbers, and flags. Packets cite it as the baseline.
- `LEARNINGS.md` — doctrine executors must not violate.
- `docs/maintainer/tickets/` — the inbox. New discoveries file tickets; the roadmap
  pulls tickets into packets, never the reverse.
