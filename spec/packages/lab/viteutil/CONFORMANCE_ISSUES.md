# lab/viteutil Conformance Issues

Status: Active  
Last Updated: 2026-02-09

| Issue ID                 | Type                 | Affected Requirements | Summary                                                                                                                                                                                                 | Status |
| ------------------------ | -------------------- | --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ |
| `LAB-VITEUTIL-ISSUE-001` | `impl-bug-candidate` | `LAB-VITEUTIL-006`    | `DevBuild` logs `cmd.Start()` failures but currently returns `nil`, so callers cannot reliably detect failed dev-start initialization. Confirm intended contract; likely should return the start error. | open   |
| `LAB-VITEUTIL-ISSUE-002` | `impl-bug-candidate` | `LAB-VITEUTIL-015`    | `InitPort(defaultPort)` currently ignores `defaultPort` and always probes with `netutil.GetFreePort(5199)`. Confirm whether caller-provided/default port intent should be honored.                      | open   |
| `LAB-VITEUTIL-ISSUE-003` | `impl-bug-candidate` | `LAB-VITEUTIL-003`    | `prep_cmd` assumes `JSPackageManagerBaseCmd` has at least one token and indexes `split_cmd[0]`; an empty base command currently panics instead of returning a recoverable error.                        | open   |
