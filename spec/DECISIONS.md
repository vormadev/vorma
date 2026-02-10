# Decisions Log

Record unresolved or adjudicated items where normative behavior is ambiguous,
contradictory, or clearly accidental.

Rules:

- Append-only during mining.
- Use `status = OPEN` for unresolved items and `status = RESOLVED` once settled.
- Use `Package Path` as `spec/packages/...` (or `cross-package` for
  multi-package contradictions).
- `OPEN` rows must keep `selected option`, `rationale`, and `evidence` as `-`.
- `RESOLVED` rows must populate `selected option`, `rationale`, and `evidence`.

| Decision ID | Date | Status | Package Path | Question | Options Considered | Selected Option | Rationale | Evidence |
| ----------- | ---- | ------ | ------------ | -------- | ------------------ | --------------- | --------- | -------- |
