# Benchmark Results

All files in this directory are raw `go test -bench` output snapshots.

Requirements:

- Keep benchmark artifacts as raw benchmark stdout only.
- Do not include process listings or host diagnostic command output.
- If host load is clearly high/unstable, do not run benchmarks; defer and note
  in `AGENT_HANDOFF.md` that benchmarking was skipped due load.

Current reference pair (clean same-window capture):

- Baseline (pre-refactor):
  `2026-02-11_154353_OLD_pre_refactor_71515bd_clean_raw.txt`
- Current branch: `2026-02-11_153905_CURRENT_clean_raw.txt`
