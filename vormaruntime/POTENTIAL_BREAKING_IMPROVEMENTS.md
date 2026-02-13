# POTENTIAL_BREAKING_IMPROVEMENTS

Potential design cleanups that may change behavior and should be discussed
before implementation.

1. Move dev-only reload machinery into a dedicated dev subpackage.
    - Scope: route/template disk reload endpoints and their wiring.
    - Benefit: cleaner production runtime surface and less accidental coupling.
