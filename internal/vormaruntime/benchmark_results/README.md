# Benchmark Results

Raw benchmark stdout snapshots for `vormaruntime`.

## Artifact Policy

- Keep only:
    - `ORIGINAL_raw.txt`
    - `SECOND_TO_LATEST_raw.txt`
    - `FRESH_raw.txt`
- Keep filenames stable; update contents in place.
- Never include host process diagnostics in these files.

## Capture Procedure

- Preserve `ORIGINAL_raw.txt`.
- Before a new capture, copy current `FRESH_raw.txt` to
  `SECOND_TO_LATEST_raw.txt`.
- Record a new `FRESH_raw.txt`.
- Use slice-based runs with `-count=3`:
    - loaders/deps/html/cold-cache set
    - actions set
    - SSR set
- Compare runs using median `ns/op` per benchmark.

## Current Capture Timestamps

- `ORIGINAL_raw.txt`: `2026-02-11 15:43:53`
- `SECOND_TO_LATEST_raw.txt`: `2026-02-12 10:20:18`
- `FRESH_raw.txt`: `2026-02-12 11:45:47`
