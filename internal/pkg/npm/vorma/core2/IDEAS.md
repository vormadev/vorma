# Core2 Architecture Ideas

- [ ] **Operation algebra**: model navigation, prefetch, revalidation, submit,
      and HMR as instances of one operation lifecycle: start, abort, complete,
      ignore stale, publish, and fail.

- [ ] **Response classification first**: classify route responses into semantic
      outcomes before applying state changes: publish route, redirect, reload
      for build skew, ignore stale response, surface error, or request
      revalidation.

- [ ] **Publication transaction**: make history, DOM/head/CSS, commit, render,
      scroll, view transition, and settlement one explicit publication pipeline
      instead of relying on scattered command array ordering.

- [ ] **Statechart-ish router**: evaluate whether explicit router modes such as
      booting, idle, navigating, submitting, refreshing, and publishing would
      make illegal states harder to represent without blocking legitimate
      overlapping work.

- [ ] **Resource graph**: explore whether routes, modules, loaders, CSS, and
      head data should be modeled as invalidated resources with dependency
      edges, so navigation and revalidation become graph updates.

- [ ] **Background revalidation model**: make background revalidation fit the
      same operation model as foreground navigation while preserving that it can
      refresh work without taking over the visible transition.

- [ ] **Reducer density cleanup**: reduce dense reducer branches by moving
      repeated operation, response, and publication decisions into real domain
      vocabulary, not generic helpers or one-line wrappers.

- [ ] **Command ordering audit**: after publication becomes explicit, audit
      command ordering so correctness is local to the publication model and not
      hidden in incidental array push order.

- [ ] **Architectural size reduction**: collapse duplicated lifecycle semantics,
      response handling density, and publication sequencing instead of chasing
      minifier tricks.

- [ ] **Size justification checkpoint**: judge the 5 kB gzip increase after the
      operation/outcome/publication cleanup; core2 should earn the size through
      clearer ownership, fewer race-prone paths, and easier future changes.
