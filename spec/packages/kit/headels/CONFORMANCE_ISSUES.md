# kit/headels Conformance Issues

Status: Active  
Last Updated: 2026-02-09

| Issue ID | Type | Affected Requirement(s) | Summary | Status |
|---|---|---|---|---|
| KIT-HEADELS-ISSUE-001 | intent-validation-gap | KIT-HEADELS-001, KIT-HEADELS-009 | Render tests assert major content presence but do not pin exact marker comment strings and full marker-boundary ordering semantics. | open |
| KIT-HEADELS-ISSUE-002 | intent-validation-gap | KIT-HEADELS-011, KIT-HEADELS-014, KIT-HEADELS-016 | `FromRaw` aliasing behavior, full `AddElements` copy/ordering semantics, and portions of exported helper-wrapper surface remain primarily source-derived. | open |
| KIT-HEADELS-ISSUE-003 | impl-bug-candidate | KIT-HEADELS-003 | `InitUniqueRules` is `sync.Once`-gated; if `Render`/`ToSortedAndPreEscapedHeadEls` runs first, later custom-rule initialization is ignored. Confirm this call-order lock-in is intended API behavior. | open |
| KIT-HEADELS-ISSUE-004 | intent-validation-gap | KIT-HEADELS-010 | Only one render-error branch is covered directly; title/rest wrapper branches and nil-input preconditions are currently source-only. | open |
