# kit/lazyget Traceability Matrix

Status: In Progress  
Last Updated: 2026-02-09

Note: source-only or partial rows are explicitly issue-backed in
`CONFORMANCE_ISSUES.md`.

| Requirement ID    | Scenario ID(s)        | Suite Family | Suite Name                          | Fixture Type    | Test File(s)                  | Pass Criteria                                                                                               | Status      | Owner         |
| ----------------- | --------------------- | ------------ | ----------------------------------- | --------------- | ----------------------------- | ----------------------------------------------------------------------------------------------------------- | ----------- | ------------- |
| `KIT-LAZYGET-001` | `KIT-LAZYGET-SCN-001` | kit/lazyget  | cache-get-once                      | unit            | `kit/lazyget/lazyget_test.go` | Repeated `Cache.Get` calls return initialized value while initializer executes once.                        | covered     | `kit/lazyget` |
| `KIT-LAZYGET-002` | `KIT-LAZYGET-SCN-002` | kit/lazyget  | cache-get-concurrency               | unit            | `kit/lazyget/lazyget_test.go` | Concurrent access preserves single initialization and stable returned value.                                | covered     | `kit/lazyget` |
| `KIT-LAZYGET-003` | `KIT-LAZYGET-SCN-003` | kit/lazyget  | cache-get-first-initializer-binding | source-contract | `(none)`                      | First-initializer binding across differing subsequent init funcs is source-owned (`KIT-LAZYGET-ISSUE-001`). | source-only | `kit/lazyget` |
| `KIT-LAZYGET-004` | `KIT-LAZYGET-SCN-004` | kit/lazyget  | cache-get-nil-initfunc              | unit            | `kit/lazyget/lazyget_test.go` | `Cache.Get(nil)` panics when initialization is invoked.                                                     | covered     | `kit/lazyget` |
| `KIT-LAZYGET-005` | `KIT-LAZYGET-SCN-005` | kit/lazyget  | cache-get-panic-sticky              | unit            | `kit/lazyget/lazyget_test.go` | Initializer panic is sticky and initializer executes once.                                                  | covered     | `kit/lazyget` |
| `KIT-LAZYGET-006` | `KIT-LAZYGET-SCN-006` | kit/lazyget  | cache-get-generic-types             | unit            | `kit/lazyget/lazyget_test.go` | Once semantics hold across tested generic value types.                                                      | covered     | `kit/lazyget` |
| `KIT-LAZYGET-007` | `KIT-LAZYGET-SCN-007` | kit/lazyget  | new-constructor-once                | unit            | `kit/lazyget/lazyget_test.go` | `New` returns callable once getter with expected value behavior.                                            | covered     | `kit/lazyget` |
| `KIT-LAZYGET-008` | `KIT-LAZYGET-SCN-008` | kit/lazyget  | new-nil-initfunc                    | unit            | `kit/lazyget/lazyget_test.go` | Getter returned by `New(nil)` panics when invoked.                                                          | covered     | `kit/lazyget` |
| `KIT-LAZYGET-009` | `KIT-LAZYGET-SCN-009` | kit/lazyget  | new-panic-sticky                    | unit            | `kit/lazyget/lazyget_test.go` | `New` panic behavior is sticky and initializer executes once.                                               | covered     | `kit/lazyget` |
