# POTENTIAL_BREAKING_IMPROVEMENTS

Potential design cleanups that may change behavior and should be discussed
before implementation.

1. Move dev-only reload machinery into a dedicated dev subpackage.
    - Scope: route/template disk reload endpoints and their wiring.
    - Benefit: cleaner production runtime surface and less accidental coupling.

2. Remove legacy compatibility exports once callers are updated.
    - Package-level `GetHeadElsInstance()` singleton accessor.
    - Legacy dev reload endpoint aliases: `DevReloadRoutesPath` and
      `DevReloadTemplatePath`.

3. Make runtime-injected template key names configurable.
    - Current injected keys are fixed (`VormaHeadEls`, `VormaBodyScripts`,
      etc.).
    - Configurable key names would avoid assumptions on behalf of application
      templates and reduce collision risk.

4. Make dev reload endpoint paths configurable.
    - Current defaults are fixed (`/__vorma/reload-routes`,
      `/__vorma/reload-template`).
    - Configurable endpoints would reduce namespace collision risk in host apps.
