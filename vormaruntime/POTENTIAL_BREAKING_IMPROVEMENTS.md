# POTENTIAL_BREAKING_IMPROVEMENTS

Potential design cleanups that may change behavior and should be discussed
before implementation.

1. Isolate dev-only endpoints and reload machinery into a dedicated subpackage.
    - Candidate scope: route/template disk reload endpoints and related wiring.
    - Benefit: clearer runtime surface and less accidental prod coupling.
    - Risk: import paths, setup order, and extension points could shift.

2. Deprecate and remove package-level `GetHeadElsInstance()`.
    - Runtime internals now use app-scoped head instances.
    - The package-level accessor remains only for compatibility and still
      exposes a legacy singleton instance.
    - Removing or changing it is likely a public API behavior break.

3. Fully unify init-time and dev-reload route synchronization semantics.
    - Init re-init now replaces parsed-path state, rebuilds nested routes
      against current parsed paths while preserving server handlers, and clears
      route-data cache.
    - Dev reload still uses `RouteRegistry.Sync`, which additionally merges
      server-only handler routes into parsed paths.
    - Aligning these flows may simplify state reasoning and reduce behavioral
      drift.

4. Consolidate loaders-route decoration lifecycle.
    - Parsed client routes are decorated at handler construction time, while
      re-init now also rebuilds nested-route registrations.
    - Collapsing to one explicit lifecycle path could reduce state-coupling
      complexity.

5. Remove legacy dev reload endpoint constant aliases.
    - Canonical names are `Dev_ReloadRoutesPath` and `Dev_ReloadTemplatePath`.
    - Legacy aliases (`DevReloadRoutesPath`, `DevReloadTemplatePath`) remain for
      compatibility.
    - Removing aliases is a potential API break for older callers.
