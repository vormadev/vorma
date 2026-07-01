# Gate Speed Profiling

Status: blocked until gate cost is a demonstrated development bottleneck

`make gate` is the release-level confidence gate. Do not weaken it to make it faster.

This ticket exists because gate-speed profiling was explicitly separated from API/docs
work. Do this only if gate cost is interfering with actual development.

Constraints:

- Correctness and coverage beat shaving seconds.
- Do not skip tests when resources are missing.
- If a step is slow, profile before changing it.
- Prefer removing duplicated work or improving tooling mechanics over narrowing coverage.

What to do:

- Measure `make gate` end to end before changing anything.
- Identify which steps dominate wall time.
- Remove duplicated work or improve mechanics where possible.
- Do not narrow coverage, skip tests, or introduce alternate weaker gates.

Done means:

- The slow path is measured.
- Any change preserves the same confidence contract.
- The gate remains the command maintainers can trust before release-level decisions.
