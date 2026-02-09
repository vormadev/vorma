# vormabuild Conformance Issues

Status: Active  
Last Updated: 2026-02-09

| Issue ID | Type               | Affected Requirements | Summary                                                                                                                           | Status |
| -------- | ------------------ | --------------------- | --------------------------------------------------------------------------------------------------------------------------------- | ------ |
| VCI-024  | impl-bug-candidate | BUILD-ROUTE-005       | Route DSL parser does not currently enforce strict required signature for route(pattern, module, ...).                            | open   |
| VCI-026  | impl-bug-candidate | BUILD-ROUTE-006       | Duplicate route-pattern collisions are currently silent last-write-wins with no diagnostics.                                      | open   |
| VCI-040  | impl-bug-candidate | BUILD-VITE-010        | Vite dev-port helper currently ignores configured/default candidate port during free-port selection.                              | open   |
| VCI-041  | impl-bug-candidate | BUILD-VITE-011        | Vite dev-start command failure is currently logged but not propagated as an error.                                                | open   |
| VCI-050  | impl-bug-candidate | BUILD-ART-004         | Client-defined loader-only route typing currently derives params/splat with action matcher runes instead of loader matcher runes. | open   |
