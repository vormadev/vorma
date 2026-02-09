# kit/mux Conformance Issues

Status: Active  
Last Updated: 2026-02-09

| Issue ID | Requirement(s) | Summary | Status |
|---|---|---|---|
| `KIT-MUX-ISSUE-001` | `KIT-MUX-015`, `KIT-MUX-020`, `KIT-MUX-024`, `KIT-MUX-035`, `KIT-MUX-040`, `KIT-MUX-041` | Intent-validation: fast-path request-store injection details, non-validation parse-input 500 mapping branch, nil-ish task-output warning branch, HTTP chain cache behavior, full ReqData helper-wrapper forwarding surface, and `InjectTasksCtxMiddleware` idempotent injection are currently source-only contracts. Add explicit conformance tests or confirm source-only acceptance. | open |
| `KIT-MUX-ISSUE-002` | `KIT-MUX-004`, `KIT-MUX-008`, `KIT-MUX-009`, `KIT-MUX-014`, `KIT-MUX-023`, `KIT-MUX-034`, `KIT-MUX-039` | Intent-validation: these requirements are partially test-backed but still rely on source-only branch details (method-matcher option propagation, live AllRoutes exposure nuance, full matcher delegation edge precedence, full fast-path gating matrix, task-handler proxy short-circuit branch surface, HTTP-route+task-middleware chain composition details, and helper no-context fallback nuances). Confirm intended branch semantics and close partial-to-covered gaps. | open |
| `KIT-MUX-ISSUE-003` | `KIT-MUX-045`, `KIT-MUX-046`, `KIT-MUX-047`, `KIT-MUX-048` | Intent-validation: nested-task edge branches and rebuild internals remain partially or fully source-backed (no-tasksCtx/empty-match branches, full proxy allocation+parallel error-log branches, live-state accessor mutability, atomic rebuild APIs, and req-data pool safety dependence on blocking `tasks.Ctx.RunParallel`). Add targeted tests or explicitly accept these as source-owned contracts. | open |
