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

3. Unify route synchronization behavior between init and live reload.
    - `devReloadRoutesFromDisk()` explicitly re-syncs merged server routes.
    - Initial `Init()` path loading follows a different path.
    - Aligning these paths may change which routes are available immediately
      after startup.

4. Remove legacy dev reload endpoint constant aliases.
    - Canonical names are `Dev_ReloadRoutesPath` and `Dev_ReloadTemplatePath`.
    - Legacy aliases (`DevReloadRoutesPath`, `DevReloadTemplatePath`) remain for
      compatibility.
    - Removing aliases is a potential API break for older callers.
