# lab/tsgen Conformance Issues

Status: Active  
Last Updated: 2026-02-09

| Issue ID              | Type                    | Affected Requirements                                                                                | Summary                                                                                                                                                                                                                        | Status |
| --------------------- | ----------------------- | ---------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------ |
| `LAB-TSGEN-ISSUE-001` | `intent-validation-gap` | `LAB-TSGEN-009`, `LAB-TSGEN-010`, `LAB-TSGEN-011`, `LAB-TSGEN-012`, `LAB-TSGEN-013`, `LAB-TSGEN-014` | Error-path behavior and helper-only APIs (`to_file` + `statements`) are currently source-derived without direct legacy test assertions. Confirm intended strictness and add targeted evidence if these contracts are critical. | open   |
| `LAB-TSGEN-ISSUE-002` | `intent-validation-gap` | `LAB-TSGEN-001`, `LAB-TSGEN-004`, `LAB-TSGEN-005`                                                    | `generate_ts_content` special branches for `TSTyperRaw` (merge exclusion + phantom rendering) and marshal-failure propagation from `formatJSValue` are currently source-verified only, without direct legacy assertions.       | open   |
