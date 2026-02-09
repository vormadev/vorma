# kit/response Conformance Issues

Status: Active  
Last Updated: 2026-02-09

| Issue ID | Type | Affected Requirement(s) | Summary | Status |
|---|---|---|---|---|
| KIT-RESPONSE-ISSUE-001 | impl-bug-candidate | KIT-RESPONSE-011 | `validateURL` currently accepts scheme-relative URLs (e.g. `//host/path`) because they are not treated as absolute with scheme. Confirm whether cross-origin scheme-relative redirects are intentionally allowed. | open |
| KIT-RESPONSE-ISSUE-002 | intent-validation-gap | KIT-RESPONSE-015, KIT-RESPONSE-018 | Head-element behavior (`AddHeadEls`/`GetHeadEls` and merged head-element ordering) is source-defined but lacks direct legacy test coverage. | open |
| KIT-RESPONSE-ISSUE-003 | intent-validation-gap | KIT-RESPONSE-001, KIT-RESPONSE-002, KIT-RESPONSE-003, KIT-RESPONSE-007, KIT-RESPONSE-008, KIT-RESPONSE-009, KIT-RESPONSE-010, KIT-RESPONSE-011, KIT-RESPONSE-012 | Commit-state transitions, redirect code-normalization fallback behavior, committed-state guard branches, and helper-only surfaces remain partially source-derived and need targeted evidence or explicit source-only acceptance. | open |
