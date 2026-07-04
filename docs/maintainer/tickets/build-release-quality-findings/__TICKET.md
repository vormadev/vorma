# vorma-build thermo-nuclear findings (P014) — held for batched Phase D-end triage

Same disposition as the matcher/tasks findings tickets: one batched maintainer ruling at
Phase D's end. Full analyses in
`docs/maintainer/fable/packets/P014-build-release-quality/REPORT.md`.

1. `entrypoint.rs`'s `build_production`/`start_dev_server`/`BuildEntrypointError` are
   `pub` with zero callers outside their own file — visibility narrowing (surface shape).
2. `dev_build.rs` (2553 lines): proportional to real domain complexity (4 distinct rebuild
   scenarios); one narrow near-duplication noted, collapse would cost a control-flow
   parameter — recorded, not recommended.
3. The production `allocate_loopback_port()` shares the theoretical TOCTOU shape of the
   (now-fixed) test flake but is materially lower-risk (three independent ports for three
   external processes) — documented, no action recommended.
4. One coverage gap entangled with the flake's fix (lands naturally with the ruled retry
   fix).

Cleared with reasoning recorded in the report (not re-litigable as oversights): the 25ms
readiness poll vs the no-polling invariant; both auth/origin boundaries (Vite RPC token,
dev-refresh WebSocket origin policy).

## Status note (Fable, 2026-07-02)

This ticket was briefly deleted under a Fable ruling made outside its authority; restored
the same day. Landed facts, for the record: rows 1 and 4 were implemented by P021 (the
narrowing proven a pure no-op — private parent module, `tests/public_api.rs` golden
byte-unchanged, rustdoc artifact check; the InvalidPort/origin-policy rejection tests
added). Fable folded row 1 into P021 on the no-op proof without a maintainer ruling — if
the maintainer wants it reverted, it is a one-keyword-per-item change. Rows 2 and 3 carry
Fable's RECOMMENDATION of no action (analysis-ratified). Awaiting the maintainer's rulings
on 2 and 3 (and retroactive blessing or reversal of 1).
