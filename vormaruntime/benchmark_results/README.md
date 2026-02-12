# Benchmark Results

This directory stores raw `go test -bench` stdout snapshots only.

## Rules

- Keep artifacts as raw benchmark stdout.
- Do not include process listings or host diagnostic command output.
- If host load is clearly unstable, skip benchmark capture and note it in
  `AGENT_HANDOFF.md`.
- Keep only three benchmark artifacts in this directory: `ORIGINAL`,
  `SECOND_TO_LATEST`, and `FRESH`.
- On each new benchmark capture, keep `ORIGINAL` unchanged.
- On each new benchmark capture, move prior `FRESH` to `SECOND_TO_LATEST`.
- On each new benchmark capture, record the new run as `FRESH`.
- Delete all intermediary benchmark captures after recording the new `FRESH`.

## Artifact Files

- `ORIGINAL_raw.txt`
- `SECOND_TO_LATEST_raw.txt`
- `FRESH_raw.txt`

## Capture Timestamps

- `ORIGINAL_raw.txt`: `2026-02-11 15:43:53`
- `SECOND_TO_LATEST_raw.txt`: `2026-02-12 10:14:35`
- `FRESH_raw.txt`: `2026-02-12 10:20:18`

Compare both:

- `FRESH` vs `ORIGINAL` for long-horizon guardrails.
- `FRESH` vs `SECOND_TO_LATEST` for immediate regression detection.
