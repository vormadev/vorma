# kit/id Conformance Issues

Status: Active  
Last Updated: 2026-02-09

| Issue ID           | Type                    | Affected Requirements      | Summary                                                                                                                                                                                                                                                                 | Status |
| ------------------ | ----------------------- | -------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ |
| `KIT-ID-ISSUE-001` | `intent-validation-gap` | `KIT-ID-007`, `KIT-ID-009` | Rare failure branches (`rand.Read` failure propagation in `New` and delegated error propagation in `NewMulti`) are currently source-derived and not directly asserted by legacy tests. Add deterministic fault-injection tests if explicit branch evidence is required. | open   |
