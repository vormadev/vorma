# vormaruntime Conformance Issues

Status: Active  
Last Updated: 2026-02-09

| Issue ID | Type | Affected Requirements | Summary | Status |
|---|---|---|---|---|
| VCI-015 | impl-bug-candidate | BR-HTML-008 | HTML render path is not nil-safe when GetRootTemplateData returns (nil, nil). | open |
| VCI-018 | impl-bug-candidate | BR-CONC-001, BR-CONC-002, BR-DEV-007 | SSR bootstrap DTO reads mutable runtime fields without lock while dev reload writes those fields under lock. | open |
| VCI-020 | impl-bug-candidate | BR-LOAD-014 | Route-data cache key uses direct normalized-pattern concatenation without tuple-boundary separators. | open |
| VCI-021 | impl-bug-candidate | BR-LOAD-015 | Route-data snapshot cache is process-global and not scoped per app instance. | open |
| VCI-022 | impl-bug-candidate | BR-CONC-003, API-GO-014 | Snapshot-style accessors currently return mutable aliases into internal runtime state. | open |
| VCI-023 | impl-bug-candidate | BR-ACT-005, API-GO-014 | Actions().SupportedMethods() currently returns the mutable backing map. | open |
| VCI-025 | impl-bug-candidate | BR-INIT-007 | Head dedupe rule state appears process-global rather than app-instance scoped. | open |
| VCI-028 | impl-bug-candidate | BR-HEAD-004 | Default-head callback failures can currently mask terminal non-render outcomes. | open |
| VCI-030 | impl-bug-candidate | BR-INIT-006 | Repeated Init() on the same app instance can preserve stale route-path entries that are no longer present in selected paths artifact. | open |
| VCI-061 | impl-bug-candidate | BR-CONC-004 | WithRLock currently exposes mutating setters through LockedVorma, so read-lock callbacks can write lock-protected runtime state. | open |
| VCI-062 | impl-bug-candidate | BR-LOAD-023 | RegisterPatternIfNeeded has a check-then-register race that can panic under concurrent calls for the same new pattern. | open |
| VCI-069 | impl-bug-candidate | BR-CONC-003, API-GO-014 | Router route-collection accessors currently return mutable backing collections. | open |
| VCI-070 | impl-bug-candidate | BR-ACT-006 | Typed GET-action HEAD fallback currently fails default parse flow with unsupported-method error. | open |
