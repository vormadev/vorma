# Effect Runtime Checklist

This is the live checklist for making Vorma's client-side runtime fully
Effect-owned where that improves correctness, lifetime ownership, cancellation,
typed failure, or architecture.

Keep this file current. Delete completed or obsolete work instead of preserving
historical notes.

## Placement Rule

- Effect-owned: runtime state, host APIs, listeners, timers, cancellation, async
  execution, resource lifetime, caches, subscriptions, and typed failures.
- Plain TypeScript: deterministic transforms, type-level facades, constants,
  simple object shaping, and pure URL/path/body helpers.

## Active Checklist

- [ ] Thin the framework adapters. `tsx/react`, `tsx/preact`, `tsx/solid`, and
      `tsx/remix` should remain framework-native at the public surface, but
      should stop duplicating store, subscription, route-sync, and link
      projection mechanics where shared adapter runtime state can handle them.

- [ ] Continue shrinking `create_client_core_effect.ts`. It should become a
      compatibility facade: acquire runtime, expose public methods, lower
      Effects to Promise/sync returns, and translate public result shapes.
      Router behavior belongs in kernel or service modules.

- [ ] Tighten the service graph. Move broad constructor options and ad hoc
      records toward explicit `Context` / `Layer` ownership when that improves
      composition. Do not add wrapper layers that merely rename one-line Effect
      calls.

- [ ] Move remaining runtime resources into scoped acquisition. Remaining
      targets are async host bridges, timer scheduling/fibers, abort-controller
      ownership, pending prestarts, HMR replacement, and any actor/fiber
      lifetime still registered after acquisition instead of as part of it.

- [ ] Tighten typed error channels. Push domain-specific tagged failures deeper
      through route preparation, route publishing, revalidation, and submission.
      Collapse them only at public adapter edges.

- [ ] Review payload decoding and route input contracts. Payload decoding is
      still hand-shaped. Decide whether Effect Schema or a local decoder layer
      should own this.

- [ ] Review retry/backoff with Effect primitives. Revalidation retry/backoff
      works, but it should be checked against `Schedule` before that subsystem
      is considered settled.

- [ ] Promote or delete remaining `ccc_effect_*` files. Keeper tests should
      become production-named tests. Swap runners should either become permanent
      parity coverage with clear names or disappear.

- [ ] Audit small helpers for fake-wrapper smell. Keep helpers that encode
      domain behavior. Inline helpers that only hide one Effect combinator or
      one direct Effect runtime call. Public compatibility lowering is fine, but
      internal runtime code should not hide Effect behind no-op aliases.

- [ ] Review `browser_view_runtime` view-transition bridging. This is legitimate
      host API plumbing, but it remains one of the most callback-shaped runtime
      regions.

- [ ] Review `route_preparer` client-loader prestart promises. The behavior is
      important, but this area still has Promise-era residue.

## Switch-Over Bar

- [x] Full Vorma core tests are green.
- [x] TypeScript typecheck is green.
- [x] TypeScript lint is green, ignoring only already-known unrelated warnings
      if they remain outside this package.
- [ ] The facade keeps shrinking instead of accumulating router logic.
- [ ] Scoped lifetime ownership keeps expanding.
- [ ] Non-Effect-shaped regions are deliberate pure helpers or public/framework
      adapters, not accidental ports of the old closure.
