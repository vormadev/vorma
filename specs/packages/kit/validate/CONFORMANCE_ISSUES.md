# kit/validate Conformance Issues

Status: Active  
Last Updated: 2026-02-09

| Issue ID | Requirement(s) | Summary | Status |
|---|---|---|---|
| `KIT-VALIDATE-ISSUE-001` | `KIT-VALIDATE-011`, `KIT-VALIDATE-012`, `KIT-VALIDATE-019`, `KIT-VALIDATE-020` | Intent-validation: low-level URL parsing internals are only partially test-backed (strict destination-kind contract details, interface-pointer dereference branch, full scalar parse edge surface, and full nested map/slice helper branch matrix). Add focused conformance tests or explicitly accept these as source-owned contracts. | open |
| `KIT-VALIDATE-ISSUE-002` | `KIT-VALIDATE-030`, `KIT-VALIDATE-032` | Intent-validation: `attemptValidation` entry branches (including nil-input shortcut) and low-level utility semantics (`safeDereference`, `getTypeState`, `isEffectivelyZero`, `safeIsNil`) are partially asserted but still rely on source-level branch interpretation. Confirm intended behavior and add direct tests if required. | open |
