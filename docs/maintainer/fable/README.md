# Fable-Managed Roadmap

This directory is the orchestration layer for getting Vorma to release quality. It is
owned by the orchestrating agent ("Fable"): Fable writes the roadmap, defines work
packets, and reviews completed work. Executor agents run individual packets.

If you are an agent landing here cold: read this file, then `LEARNINGS.md`, then
`STATE.md`, then the packet you were assigned. Do not start work without reading all four.
`AGENTS.md` at the repo root binds you as well.

## Layout

- `ROADMAP.md` — the sequenced plan to release: phases, packets, and status.
- `STATE.md` — what is true right now: gate status, recorded benchmark tables, open flags.
  Perishable by design; updated whenever a packet lands.
- `LEARNINGS.md` — durable technical doctrine and hard-won facts. Not history. If a fact
  no longer matters going forward, it does not belong here.
- `packets/Pnnn-slug/` — one directory per work packet:
    - `INSTRUCTIONS.md` — written by Fable. Self-contained: an executor needs no chat
      history and no other conversation context.
    - `REPORT.md` — written by the executor when done (template below).
    - `REVIEW.md` — written by Fable after reviewing: accepted, or rework items.

Relationship to `docs/maintainer/tickets/`: tickets are the inbox for discovered and
future work. The roadmap consumes tickets into sequenced packets when their turn comes.
Discovering new work while executing a packet means filing a ticket, not expanding the
packet.

## The Packet Protocol

Every packet's `INSTRUCTIONS.md` contains: context (why this work exists and what
surrounds it), exact scope, hard constraints, escalation triggers, and a definition of
done with verification commands and expected results.

The rules that make this work — these bind every executor:

1. **Stopping and reporting a blocker is success. Improvising past a constraint is
   failure.** If the packet cannot be completed as specified, stop and write the blocker
   into `REPORT.md`. A packet that comes back honestly incomplete is worth more than one
   that comes back "done" with silent judgment calls.
2. **No semantic changes, ever, inside a packet — unless the packet explicitly grants
   them.** Observable behavior (matching results, cache retention, cancellation timing
   classes, error selection, panic behavior, public API shape) is frozen unless
   `INSTRUCTIONS.md` says otherwise. If you believe a semantic change is needed or you
   found what looks like a bug: red failing test first if cheaply possible, then escalate
   in `REPORT.md`. The maintainer decides.
3. **The gate is the bar.** A packet is not done with a red gate. Never weaken a test, a
   lint, a loom model, or a bench recording to get green.
4. **Benchmarks are recorded only through their `make bench-*` targets** (pure redirect of
   the harness output). Never hand-edit a `bench.results.txt`. Always paste before/after
   numbers in the report, compared against the baselines in `STATE.md`.
5. **Verified code is shipped code.** Changes to concurrency protocol code in
   `vorma-tasks` require the loom models to still pass (`make loom-tasks`), and new
   protocol transitions require new models. Do not add fast paths the loom build cannot
   see.

## REPORT.md Template

```
# Pnnn Report

## What changed
(files + one-line purpose each)

## Decisions made
(every judgment call, with justification — an empty section is a claim that zero
judgment calls were made)

## Gate results
(pasted output: workspace tests, clippy, fmt check, loom, plus any packet-specific
verification commands)

## Benchmarks
(pasted before/after where applicable, with comparison table)

## Escalations / open questions
(anything the maintainer must rule on)

## Discovered out-of-scope work
(tickets filed, with paths)
```
