# kit/securebytes Conformance Issues

Status: Active  
Last Updated: 2026-02-09

| Issue ID                    | Type                    | Affected Requirements | Summary                                                                                                                                                                                                                                                           | Status |
| --------------------------- | ----------------------- | --------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ |
| `KIT-SECUREBYTES-ISSUE-001` | `intent-validation-gap` | `KIT-SECUREBYTES-006` | The `invalid plaintext: too short` branch is currently source-derived and not directly exercised by legacy tests because most malformed inputs fail earlier in authenticated decrypt. Confirm branch-level testing needs or accept as source-owned edge handling. | open   |
