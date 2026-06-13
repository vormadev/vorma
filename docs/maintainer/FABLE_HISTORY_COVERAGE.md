# Fable History Coverage Ledger

This ledger records the raw Claude/Fable JSONL history files processed for the durable
maintainer notes. Coverage is tracked by original file and original JSONL line range.

The goal is not to preserve a changelog. The goal is to prove which source material was
read and to make it possible to audit what was captured without reopening every raw
history file.

## Source Files

Top-level histories:

- `36126369-0da4-45a2-83f9-65bd07a35805.jsonl`: 1,808 lines, 4.3 MB.
- `9e3bfc5a-bb96-4b96-ad2c-24995f8ad7d7.jsonl`: 11,586 lines, 28 MB.
- `f941344a-12be-4bf4-823c-0b3b17b19f28.jsonl`: 178 lines, 292 KB.
- `6b4ce1cd-2179-466e-be7f-027821c2e834.jsonl`: 25 lines, 36 KB.

Subagent histories under `36126369-0da4-45a2-83f9-65bd07a35805/subagents`:

- `agent-a56a731c605a9c4f5.jsonl`: 92 lines.
- `agent-af74f2f73213c28fc.jsonl`: 43 lines.
- `agent-a3af29cef13d1db7b.jsonl`: 77 lines.
- `agent-a7544f0c236dd2083.jsonl`: 125 lines.
- `agent-a8cea6ae06289f90c.jsonl`: 84 lines.
- `agent-ac396fa987203cc53.jsonl`: 129 lines.
- `agent-a86091913f194f814.jsonl`: 75 lines.
- `agent-a287fa6b9c7e98fac.jsonl`: 26 lines.
- `agent-a26c9de72c4683f98.jsonl`: 115 lines.
- `agent-aaccdbe603579d785.jsonl`: 259 lines.
- `agent-a4bb7e54fcfa92eea.jsonl`: 42 lines.
- `agent-ae04ed7ce37dd5979.jsonl`: 43 lines.
- `agent-a2f26614b39113d61.jsonl`: 69 lines.
- `agent-a28e9f64ff5de8a6f.jsonl`: 61 lines.
- `agent-a15a5d61cba343d31.jsonl`: 60 lines.
- `agent-a0256a944f6e46952.jsonl`: 98 lines.
- `agent-ad2315da3be2d640c.jsonl`: 62 lines.
- `agent-a1bc1e4453f66cd77.jsonl`: 61 lines.

Subagent histories under `9e3bfc5a-bb96-4b96-ad2c-24995f8ad7d7/subagents`:

- `agent-a053839bde3f7f79a.jsonl`: 133 lines.
- `agent-a9dfc0c2041aff3bf.jsonl`: 129 lines.
- `agent-a144e40f16b92520d.jsonl`: 232 lines.
- `agent-a9053e0931cb493b2.jsonl`: 146 lines.

Sidecar artifacts:

- `f941344a-12be-4bf4-823c-0b3b17b19f28/tool-results/b4bu8cn6m.txt`:
  persisted 49.5 KB command output from the Node/PATH investigation.
- `*.meta.json` files beside each subagent history: subagent metadata to inspect with
  the corresponding subagent JSONL.

Total JSONL coverage scope after including subagents: 15,758 lines.

## Coverage Log

| File | Lines | Status | Notes file updates |
| --- | --- | --- | --- |
| `6b4ce1cd-2179-466e-be7f-027821c2e834.jsonl` | 1-25 | complete | `FABLE_TRANSCRIPT_NOTES.md` / Environment and Tooling Context |
| `f941344a-12be-4bf4-823c-0b3b17b19f28.jsonl` | 1-178 | complete | `FABLE_TRANSCRIPT_NOTES.md` / Permissions, Hidden Memory, and Environment Context |
| `36126369-0da4-45a2-83f9-65bd07a35805.jsonl` | 1-1,808 | complete | `FABLE_REGRESSION_NOTES.md`; `FABLE_WORKSTREAMS.md`; `FABLE_TRANSCRIPT_NOTES.md` |
| `9e3bfc5a-bb96-4b96-ad2c-24995f8ad7d7.jsonl` | 1-11,586 | complete | `FABLE_ARCHITECTURE_CAMPAIGN_NOTES.md`; `FABLE_API_BOARD_NOTES.md`; `FABLE_MATCHER_NOTES.md`; `FABLE_TASKS_NOTES.md`; `FABLE_ARCHITECTURE_NOTES.md`; `FABLE_REGRESSION_NOTES.md` |
| `36126369-0da4-45a2-83f9-65bd07a35805/subagents/agent-a0256a944f6e46952.jsonl` | 1-98 | complete | `FABLE_REGRESSION_NOTES.md` / Cleanup And Performance Findings |
| `36126369-0da4-45a2-83f9-65bd07a35805/subagents/agent-a15a5d61cba343d31.jsonl` | 1-60 | complete | `FABLE_REGRESSION_NOTES.md` / TypeScript And Vite Findings |
| `36126369-0da4-45a2-83f9-65bd07a35805/subagents/agent-a1bc1e4453f66cd77.jsonl` | 1-61 | complete | `FABLE_REGRESSION_NOTES.md` / Runtime Request-Path Regressions Found |
| `36126369-0da4-45a2-83f9-65bd07a35805/subagents/agent-a26c9de72c4683f98.jsonl` | 1-115 | complete | `FABLE_REGRESSION_NOTES.md` / Build And Dev-Loop Regressions Found; Test Coverage Warning |
| `36126369-0da4-45a2-83f9-65bd07a35805/subagents/agent-a287fa6b9c7e98fac.jsonl` | 1-26 | interrupted after evidence gathering | `FABLE_REGRESSION_NOTES.md` / Interrupted Subagents |
| `36126369-0da4-45a2-83f9-65bd07a35805/subagents/agent-a28e9f64ff5de8a6f.jsonl` | 1-61 | complete | `FABLE_REGRESSION_NOTES.md` / Build And Dev-Loop Fixes Later Rechecked |
| `36126369-0da4-45a2-83f9-65bd07a35805/subagents/agent-a2f26614b39113d61.jsonl` | 1-69 | interrupted after evidence gathering | `FABLE_REGRESSION_NOTES.md` / Interrupted Subagents |
| `36126369-0da4-45a2-83f9-65bd07a35805/subagents/agent-a3af29cef13d1db7b.jsonl` | 1-77 | complete | `FABLE_REGRESSION_NOTES.md` / Cleanup And Performance Findings |
| `36126369-0da4-45a2-83f9-65bd07a35805/subagents/agent-a4bb7e54fcfa92eea.jsonl` | 1-42 | interrupted after evidence gathering | `FABLE_REGRESSION_NOTES.md` / Runtime Fixes Later Partially Rechecked; Interrupted Subagents |
| `36126369-0da4-45a2-83f9-65bd07a35805/subagents/agent-a56a731c605a9c4f5.jsonl` | 1-92 | complete | `FABLE_REGRESSION_NOTES.md` / High-Level Refactor Assessment |
| `36126369-0da4-45a2-83f9-65bd07a35805/subagents/agent-a7544f0c236dd2083.jsonl` | 1-125 | complete | `FABLE_REGRESSION_NOTES.md` / Build And Dev-Loop Regressions Found |
| `36126369-0da4-45a2-83f9-65bd07a35805/subagents/agent-a86091913f194f814.jsonl` | 1-75 | complete | `FABLE_REGRESSION_NOTES.md` / Runtime Request-Path Regressions Found |
| `36126369-0da4-45a2-83f9-65bd07a35805/subagents/agent-a8cea6ae06289f90c.jsonl` | 1-84 | complete | `FABLE_REGRESSION_NOTES.md` / TypeScript And Vite Findings |
| `36126369-0da4-45a2-83f9-65bd07a35805/subagents/agent-aaccdbe603579d785.jsonl` | 1-259 | complete | `FABLE_REGRESSION_NOTES.md` / Runtime Request-Path Regressions Found; Build And Dev-Loop Regressions Found |
| `36126369-0da4-45a2-83f9-65bd07a35805/subagents/agent-ac396fa987203cc53.jsonl` | 1-129 | complete | `FABLE_REGRESSION_NOTES.md` / Runtime Request-Path Regressions Found |
| `36126369-0da4-45a2-83f9-65bd07a35805/subagents/agent-ad2315da3be2d640c.jsonl` | 1-62 | complete | `FABLE_REGRESSION_NOTES.md` / Build And Dev-Loop Regressions Found |
| `36126369-0da4-45a2-83f9-65bd07a35805/subagents/agent-ae04ed7ce37dd5979.jsonl` | 1-43 | complete | `FABLE_REGRESSION_NOTES.md` / Cleanup And Performance Findings; Test Coverage Warning |
| `36126369-0da4-45a2-83f9-65bd07a35805/subagents/agent-af74f2f73213c28fc.jsonl` | 1-43 | complete | `FABLE_REGRESSION_NOTES.md` / Build And Dev-Loop Regressions Found |
| `9e3bfc5a-bb96-4b96-ad2c-24995f8ad7d7/subagents/agent-a053839bde3f7f79a.jsonl` | 1-133 | complete | `FABLE_ARCHITECTURE_NOTES.md` / Core `vorma` Crate Pressure Points |
| `9e3bfc5a-bb96-4b96-ad2c-24995f8ad7d7/subagents/agent-a144e40f16b92520d.jsonl` | 1-232 | complete | `FABLE_ARCHITECTURE_NOTES.md` / Repo-Wide Hygiene Survey Findings To Reconcile |
| `9e3bfc5a-bb96-4b96-ad2c-24995f8ad7d7/subagents/agent-a9053e0931cb493b2.jsonl` | 1-146 | interrupted before final report | `FABLE_ARCHITECTURE_NOTES.md` / Architecture Survey Subagents |
| `9e3bfc5a-bb96-4b96-ad2c-24995f8ad7d7/subagents/agent-a9dfc0c2041aff3bf.jsonl` | 1-129 | interrupted before final report | `FABLE_ARCHITECTURE_NOTES.md` / Architecture Survey Subagents |
| `f941344a-12be-4bf4-823c-0b3b17b19f28/tool-results/b4bu8cn6m.txt` | 1-1,405 | complete | `FABLE_TRANSCRIPT_NOTES.md` / Environment and Tooling Context |
| subagent `*.meta.json` files | all 22 files | complete | `FABLE_TRANSCRIPT_NOTES.md` / Subagent Corpus Map |

## Main `9e3bfc5a...` Line-Range Ledger

The large top-level history was processed in concrete ranges after subagent coverage was
finished. The content has encrypted thinking/signature blobs and large edited-file
attachments; those lines were still counted in the raw range coverage, while the durable
notes preserve the human-readable messages, tool actions/results, edited-file facts, and
current-code reconciliation.

| Lines | Status | Durable notes |
| --- | --- | --- |
| 1-700 | complete | `FABLE_ARCHITECTURE_NOTES.md`; `FABLE_REGRESSION_NOTES.md`; `FABLE_ARCHITECTURE_CAMPAIGN_NOTES.md` |
| 701-2,200 | complete | `FABLE_REGRESSION_NOTES.md`; `FABLE_ARCHITECTURE_CAMPAIGN_NOTES.md` |
| 2,201-4,300 | complete | `FABLE_ARCHITECTURE_CAMPAIGN_NOTES.md` |
| 4,301-6,500 | complete | `FABLE_ARCHITECTURE_CAMPAIGN_NOTES.md`; `FABLE_API_BOARD_NOTES.md` |
| 6,501-8,600 | complete | `FABLE_API_BOARD_NOTES.md`; `FABLE_MATCHER_NOTES.md` |
| 8,601-10,550 | complete | `FABLE_MATCHER_NOTES.md` |
| 10,551-11,586 | complete | `FABLE_MATCHER_NOTES.md`; `FABLE_TASKS_NOTES.md` |

## Reconciliation Log

| Topic | Source transcript lines | Repo files checked | Result |
| --- | --- | --- | --- |
| Agent Node version drift | `6b4ce1cd-2179-466e-be7f-027821c2e834.jsonl` lines 1-25 | `REMINDERS.md` sandbox/pnpm reminder checked separately | The transcript proves an inherited agent shell can report stale tool versions before a restart; do not infer the maintainer's environment from one agent shell result. |
| Global Claude permissions cannot dynamically scope to current repo | `f941344a-12be-4bf4-823c-0b3b17b19f28.jsonl` lines 1-111 | Current `AGENTS.md` rules supplied in prompt; no out-of-repo settings edited by this pass | The durable rule is global deny for globally forbidden operations and per-repo/project mechanisms for additive write permissions. |
| Node version drift root cause | `f941344a-12be-4bf4-823c-0b3b17b19f28.jsonl` lines 112-178 and `6b4ce1cd-2179-466e-be7f-027821c2e834.jsonl` lines 1-25 | Current shell not mutated | Agent PATH had stale nvm version directories with old versions first; the effective test is what the agent process sees after restart, not what the maintainer's terminal reports before restart. |
| Early build/dev-loop regression reports vs later fixes | `36126369-0da4-45a2-83f9-65bd07a35805/subagents/*.jsonl` | Current code reconciliation still pending for several non-core findings | The transcript contains both original confirmed regressions and later rechecks that reported fixes. Notes separate original issue from later fixed status. |
| 361 main workstream sequence | `36126369-0da4-45a2-83f9-65bd07a35805.jsonl` lines 1-1,808 | Existing maintainer docs and current notes, not code yet | Captured regression restoration, in-process harness, wire fixtures, tsgen matrix, frontend hardening, e2e proof, false leverage claim correction, and hidden-memory incident. |
| `9e3bfc5a...` architecture campaign vs current docs | lines 1-4,801 | `CURRENT_PLAN.md`; `CURRENT_PLAN_2.md`; `CURRENT_PLAN_3.md`; `ARCHITECTURE.md`; `GO_PARITY_AUDIT.md`; `Makefile`; current config/build code | Captured the review-to-restoration-to-contract-extraction-to-collapse sequence. `CURRENT_PLAN_3.md` is a milestone, not the newest universal truth after the later API/Board/matcher/tasks work. |
| API campaign and Board pressure-test state | lines 4,810-8,600 | `API_DESIGN.md`; `PRESSURE_TEST_CENSUS.md`; `crates/vorma/src/config.rs`; TypeScript client-loader types | Current truth: `api_base` is gone, `public_static_base` remains, scoped middleware is declarative, and `ClientLoaderServerState.viewData` was never missing. Older statements to the contrary are superseded. |
| Matcher overlap/specificity and API-mount removal | lines 8,462-10,935 | `crates/vorma-matcher/src/lib.rs`; `crates/vorma-matcher/src/overlap.rs`; `crates/vorma-contract/src/graph_validation.rs`; `PRESSURE_TEST_CENSUS.md` | Current matcher API exposes `FlatMatcher`, `NestedMatcher`, `find_overlap`, and `compare_specificity`. API mount removal depends on exact overlap witnesses and one public specificity ordering. |
| Tasks review unresolved items | lines 10,936-11,586 | `crates/vorma-tasks/src/task.rs`; `crates/vorma-tasks/src/key.rs`; `Makefile`; Board repo task call sites | Current code still has `Task::new(Duration, ...)`, one-poller `run_parallel`, and FxHash fingerprints. The transcript's forward package is constructor split, singleflight, spawned `run_parallel`, and SipHash restoration. |
