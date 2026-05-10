# Core4 Notes

Core4 is a clean-room rewrite. Baseline-core is the behavioral authority, but
core4 must not copy baseline-core or core2/core3 implementation structure.

Production runtime implementation lives in `core.ts`. TypeScript-only
declarations live in `types.ts`. Dev-only runtime, including HMR, lives in
`dev.ts` and must only be imported behind `import.meta.env.DEV`.

The architectural bet is concrete router slots, not generic operations:

- `active_route`: the one boot/navigation/popstate/revalidation route update.
- `prefetch`: the at-most-one inert prefetch.
- `refresh`: freshness demand, waiters, retry timers, and running revalidation.
- `publication`: the currently crossing commit boundary.
- `submissions`: API requests, dedupe, redirects, and refresh demand.

The browser host is an explicit long-lived runtime object. The pure model owns
policy transitions; the runtime owns mutable browser resources, waiters, module
caches, listeners, and effect execution.

After construction, router model changes should happen only by accepting pure
transitions. Runtime code may update browser/resource fields, but production
route state must not be patched directly.

Work state is derived from these slots, not tracked separately. Superseded API
submissions may remain in the model until their own outcome arrives, but only
`running` submissions appear in work state/projection.

Publication plans own history, transition-hook, and scroll policy. The browser
host should apply the selected history action, optional hooks, DOM work, commit
token, render work, and scroll plan mechanically; it should not rediscover
navigation, popstate, or same-document policy.

Publication commits must also be the boundary where matching pending navigation
work disappears. Do not repair route/work atomicity with per-adapter snapshot
guards; the core commit should already expose the committed route with
`work.navigation` cleared.

Refresh demand uses a one-past route sequence. A route that was already running
when the demand was created must not satisfy that demand; the next route or
revalidation can.

Route build-skew effects carry the callback notification payload. Hosts should
deliver that payload; they should not reconstruct current route/work or default
behavior.

Keep checking this against the baseline full gate. Do not create a parallel
acceptance suite. Small tests are useful only when they protect new primitive
machinery.

---

NEXT STEP STATEMENT:

```
I would not restart, and I would not import core3’s machinery. Core4 is the
candidate now. But core3 should be used as a design conscience: where core4
still has runtime branches making policy decisions, we should ask whether core3
already taught us a cleaner “classify fact -> transition -> effect” shape.

The pass I’d do next:

1. Read core3 again with a very narrow extraction lens. Pull lessons, not code.
   Especially classification boundaries, ownership names, publication
   transaction shape, and stale completion rules.

2. Self-review core4 against those lessons. Mark places where `Core4Runtime` is
   deciding policy that could be pure transition logic instead. Only change
   spots where the result removes branching or shrinks prod bytes.

3. Move real HMR logic into `dev.ts`. I agree with this. `core.ts` should have
   only a tiny dynamic `import.meta.env.DEV` bridge. The HMR update/reloader
   code belongs in the dev chunk.

4. Clean type/lint/editor residue. Full gate passed, but we should still sweep
   IDE-style issues and remove anything awkward left over from the sprint.

5. Measure after every meaningful change. `make build-ts`, gzip JS, and likely
   `make gate` after the cleanup pass. No “architecture purity” change gets to
   sneak in without a size check.

The key rule: no new generic framework. No core3 operation-rights transplant.
Use core3 to make core4 more inevitable, not more elaborate.
```
