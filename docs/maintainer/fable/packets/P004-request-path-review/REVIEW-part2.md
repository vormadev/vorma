# P004 Part 2 Review

Reviewer: Fable (orchestrator). Date: 2026-07-01. Executor: Sonnet 5 subagent working from
Fable's ANALYSIS.md.

## Verdict

**Part 2 accepted — and with it, P004 as a whole is closed and Phase B is complete**
(pending the e2e confirmation noted below, which completed green during this review). The
measurement discipline was the best of any packet so far: interleaved A/B schedules,
control-row attribution checks on every step, two honest non-implementations proven rather
than hand-waved, and a byte-identity harness invented for the packet's own verification
needs. Every handler-reaching row improved 6–27% with response bytes proven identical.

## Independent verification performed (Fable)

- **Change surface:** exactly the five claimed engine files (+345/−99) plus the recording
  and STATE.md; scratch byte-capture harness confirmed deleted; no API signature changes
  (new items are `pub(crate)`).
- **Code review of the two correctness-sensitive diffs:**
    - The `HtmlScriptSafeJsonFormatter` is airtight: the five escape targets can only
      occur inside JSON string fragments (never in structural output or serde's own
      backslash escapes), `write_string_fragment` boundary math is correct
      (`char_indices` + `len_utf8`), pass-through slices delegate to `CompactFormatter`,
      and the UTF-8 reassembly argument holds. The executor verified byte-equality against
      the reference algorithm on adversarial inputs AND via the existing worst-case test.
    - The single-invocation fast path is strictly equivalent: nothing ever ran
      concurrently between the old spawn and its immediate join, so inline polling is
      indistinguishable for N=1; the parallel contract constrains N≥2 and is untouched;
      commit rules are shared through the extracted `commit_invocation_output` (the same
      code, moved, not duplicated); panic and drop-cancellation reach the same observable
      points (verified by the executor against the existing cancellation and
      sibling-ordering tests).
- **Gates re-run by Fable:** fmt clean, clippy `-D warnings` clean, workspace tests +
  doctests zero failures, loom 7/7.
- **Bench reproducibility:** an independent direct run reproduced every row inside the
  improved band (several rows faster still on an idler box) — the wins are real, not
  recording artifacts.
- **Behavioral end-to-end:** full `make e2e-smoke` re-run green over the optimized engine
  — real browsers consuming real rendered documents across all three adapters plus the dev
  scenarios.
- **STATE.md:** engine section correctly updated to the Part 2 table with per-step
  summary.

## Ruling on the disclosed judgment call (step 2)

The executor kept the clone-tree collapse despite a mixed A/B (−4.8 to −6.8% on
param-carrying routes; +2.7 to +5.1% on param-free routes) and asked for review. **Fable
accepts the keep**: the regressing readings sit inside the session's demonstrated ±1–9%
noise band while the wins sit outside it; the mixed sign has a coherent mechanism (empty
`Params`/`SplatValues` clones were already free, so param-free routes had nothing to
save); the change is a strict allocation reduction matching the LEARNINGS hot-path
doctrine; and the final Part1→Part2 table shows every affected row net-improved. The
doctrine's revert rule exists to kill changes that do not measurably help — this one
measurably helps where there is anything to help with. The maintainer may overrule; the
revert is isolated if so.

## Findings

Exhaustive plain list, per AGENTS.md review rules:

1. The executor deferred filing tickets for the steps-4/5 design questions, reasoning that
   the framing was itself a maintainer call. The tickets doctrine makes tickets the
   default home for discovered design questions — a ticket can state an open question
   without pre-judging it. Fable filed both at review
   (`tickets/engine-view-output-ownership/`,
   `tickets/contract-borrowed-element-construction/`). No rework needed.
2. The report guardrail recurred (REPORT-part2.md placed by Fable from the executor's
   returned content, provenance noted) — same harness behavior as Part 1, process friction
   only.

No other issues found.

## Consequence

P004 closes with the engine baseline OPTIMIZED and recorded: every handler-reaching row
−6.4% to −26.9% (resource rows now ~17–20µs, static view 43.9µs, 4-deep chain 55.8µs),
response bytes proven identical, all contracts intact. **Phase B (standing reviews) is
complete.** Next per the roadmap: Phase C (board completion — census feature clusters, the
F-17-ruled task-teaching packet, prod-build packet), authored by Fable.
