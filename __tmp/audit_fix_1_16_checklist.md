# Audit Fix Checklist (Items 1-16)

- [x]   1. Replace public `vorma.Vorma` alias leakage with explicit facade.
- [x]   2. Stop exposing mutable internal routers in public API.
- [x]   3. Unify client root-element ID contract with server-configured root ID.
- [x]   4. Make dev reload mutation endpoints method-safe (no GET mutation).
- [x]   5. Remove process-global mode/port/cache policy from instance behavior.
- [x]   6. Fail fast on invalid explicitly-set dev port inputs while allowing
       empty dev `PORT` to fall back to framework default base-port selection.
- [x]   7. Stop silently trimming/dropping/de-duping route definition patterns.
- [x]   8. Fail unresolved route declarations in dev too (no warn-and-ignore
       default).
- [x]   9. Treat invalid redirect targets as explicit errors, not silent ignore.
- [x]   10. Simplify navigation outcome/state-machine layering.
- [x]   11. Remove duplicated adapter route-outlet store logic across
        frameworks.
- [x]   12. Simplify route-outlet state canonicalization logic.
- [x]   13. Reduce DI/normalizer-heavy build pipeline internals.
- [x]   14. Remove unused `I` generic from `ActionFunc`.
- [x]   15. Align Vorma getter thread-safety docs and locking behavior.
- [x]   16. Remove public `client/__internal` export surface.

## Follow-up Regression Fixes

- [x] 2026-02-18: Dev startup panic in `/internal/site` when `PORT` is empty.
      Fixed by restoring dev empty-`PORT` fallback behavior while keeping
      fail-fast on invalid explicitly-set `PORT`. Regression test:
      `wave/env_test.go`:
      `TestMustGetPortDevUsesFallbackBasePortWhenPortMissing`.
