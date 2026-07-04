# P020 — app! macro: accept bare/imported state-type names (ruled footgun fix)

Read `docs/maintainer/fable/README.md` (the packet protocol binds you), then
`LEARNINGS.md`, `STATE.md`, repo-root `AGENTS.md`, `docs/maintainer/REMINDERS.md`. Primary
input: Finding 1 in `docs/maintainer/tickets/vorma-release-quality-findings/` (P015's
proven repros and the verified fix pattern).

## Context and authority

P015 proved empirically that `vorma::app!` rejects bare local state-type names,
use-imported names, and `self::`-qualified paths — only `crate::`-anchored paths compile,
because the `$state:ty` tokens resolve inside the generated nested module. Every in-repo
caller avoids this only by unwritten convention. Fable authorized the fix at the P015
review under the no-doc-smoothed-footguns doctrine: this packet's grant is exactly the
verified anchor-alias pattern — emit `type __VormaAppState = $state;` alongside the
generated module and reference `super::__VormaAppState` inside — which purely WIDENS
accepted input (all currently-compiling callers unaffected).

## Scope

1. **Red pins first (trybuild):** a bare local state name, a use-imported name, and a
   `self::`-qualified name each currently fail — pin one representative as a COMPILE-PASS
   test target that goes from red to green with the fix (trybuild pass tests, or an
   ordinary compiled test module exercising `app!` with a bare name; choose the shape the
   existing macro test suites use). Existing callers' forms keep compiling (the full
   workspace build is that pin).
2. **Implement the anchor-alias fix** in `vorma-macros`' app-declaration expansion, per
   the verified pattern. The alias is double-underscore-prefixed plumbing (E-PLUMBING
   class); ensure it does not appear in rustdoc (doc(hidden) or private as appropriate)
   and does not collide with user items (verify the name is reserved-shaped).
3. **Docs:** P015's interim warning on `app!` converts to a plain statement that any
   in-scope type path works; the P015 doctest can now use a bare name if that reads better
   (keep `()` examples where simplest).
4. On completion: update Finding 1's entry in `vorma-release-quality-findings` to
   RESOLVED-with-citation (do not delete the ticket — its other findings remain held for
   the batch triage).

## Hard constraints

- The change is EXACTLY the accepted-input widening — no other macro behavior, expansion
  shape, or public surface changes. Generated-module semantics (P015's newly documented
  contract) stay identical for existing callers.
- Tests never cheat; trybuild pins rebless only for genuinely changed diagnostics.
- Scoped fmt writes only; no git actions; no network installs; unexpectedly dirty unowned
  files are escalations, never cleanup. Another executor may be working concurrently in
  packages/ (TS) — do not touch that tree.

## Definition of done

- The three previously-failing forms compile (pinned); all existing callers unchanged and
  green; workspace tests + doctests, clippy `-D warnings`, fmt check, doc build
  `-D warnings`, loom untouched-green.
- REPORT.md content delivered IN the final message body (the report-file guardrail fires
  for every executor).
