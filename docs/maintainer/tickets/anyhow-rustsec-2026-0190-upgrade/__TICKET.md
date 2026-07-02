# Upgrade anyhow past RUSTSEC-2026-0190 (unsound Error::downcast_mut)

Discovered by the P001 gate closeout run (2026-07-01, Linux): `cargo audit` reports an
"unsound" advisory against `anyhow 1.0.102` in the workspace `Cargo.lock`:

```
Crate:     anyhow
Version:   1.0.102
Warning:   unsound
Title:     Unsoundness in `Error::downcast_mut()`
Date:      2026-06-25
ID:        RUSTSEC-2026-0190
URL:       https://rustsec.org/advisories/RUSTSEC-2026-0190
```

`make rust-policy` is green — cargo-audit treats unsound advisories as a non-fatal warning
("1 allowed warning found") and `cargo deny check advisories` passes — so this is not a
gate blocker. It should still be cleared rather than left to sit.

## Task

- Check the advisory for the patched anyhow release (the advisory was published
  2026-06-25; a fixed version may already exist or land shortly).
- `cargo update -p anyhow` to the fixed version (this is a Cargo.lock-only change unless a
  semver bump is required).
- Re-run `make rust-policy` and confirm the warning is gone.

## Verification

- `cargo audit` reports no warnings for anyhow.
- `make rust-policy` green.
