# TsDrafter const rendering is not oxfmt-idempotent

Found by P009 (2026-07-02). `TsDrafter::add_const`
(`crates/vorma-contract/src/tsgen.rs:99-105`) always renders multi-line quoted-key JSON
for array/object const values; the repo's oxfmt config wants short arrays single-line and
identifier-safe keys unquoted. Consequence: every regeneration of a consumer's
`vorma.gen.ts` (e.g. board's) produces oxfmt-nonconformant output and needs a follow-up
scoped `oxfmt --write` on that file before `ts-fmt-check` passes.

This was pre-existing but latent: the committed board `vorma.gen.ts` had been
hand-normalized once and not regenerated since; P009's new exported consts exposed it
freshly.

## Task

Make the generated output oxfmt-stable at generation time (in `vorma-contract`'s drafter
rendering — match the repo's own formatting conventions for short arrays and identifier
keys), so regeneration never dirties the fmt gate. Verify by regenerating board's file and
running `oxfmt --check` on it with zero follow-up write.
