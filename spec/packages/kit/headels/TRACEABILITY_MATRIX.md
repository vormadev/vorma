# kit/headels Traceability Matrix

Status: Draft  
Last Updated: 2026-02-09  
Applies To: Requirement-to-test traceability for `kit/headels`

| Requirement ID | Scenario ID(s) | Suite Family | Suite Name | Fixture Type | Test File(s) | Pass Criteria | Status | Owner |
|---|---|---|---|---|---|---|---|---|
| KIT-HEADELS-001 | KHS-001,KHS-007 | kit-headels | go_package_unit | source+legacy-tests | kit/headels/headblocks_test.go | Marker construction is source-defined; tests currently validate rendered content but not exact marker strings (`KIT-HEADELS-ISSUE-001`). | partial | kit/headels |
| KIT-HEADELS-002 | KHS-002 | kit-headels | go_package_unit | source+legacy-tests | kit/headels/headblocks_test.go | Default and custom unique-rule initialization, including deduped rule table population, is verified. | covered | kit/headels |
| KIT-HEADELS-003 | KHS-002 | kit-headels | go_package_unit | source+legacy-tests | kit/headels/headblocks_test.go | Once-only behavior and input non-mutation are validated; call-order constraint for custom rules remains issue-backed (`KIT-HEADELS-ISSUE-003`). | partial | kit/headels |
| KIT-HEADELS-004 | KHS-003 | kit-headels | go_package_unit | source+legacy-tests | kit/headels/headblocks_test.go | Hashing behavior over tag/attr/content variants is validated via collision checks. | covered | kit/headels |
| KIT-HEADELS-005 | KHS-003 | kit-headels | go_package_unit | source+legacy-tests | kit/headels/headblocks_test.go | Rule matching across regular/trusted/boolean attributes is directly verified, including multi-boolean requirements. | covered | kit/headels |
| KIT-HEADELS-006 | KHS-004 | kit-headels | go_package_unit | source+legacy-tests | kit/headels/headblocks_test.go | Rule-key dedupe and last-write-wins replacement behavior is verified with title/description duplicate cases. | covered | kit/headels |
| KIT-HEADELS-007 | KHS-004,KHS-005 | kit-headels | go_package_unit | source+legacy-tests | kit/headels/headblocks_test.go | Hash-fallback dedupe, mixed-content dedupe, and nil-element skipping are directly validated. | covered | kit/headels |
| KIT-HEADELS-008 | KHS-006 | kit-headels | go_package_unit | source+legacy-tests | kit/headels/headblocks_test.go | Title/meta/rest classification and escaped-element output behavior are verified. | covered | kit/headels |
| KIT-HEADELS-009 | KHS-007 | kit-headels | go_package_unit | source+legacy-tests | kit/headels/headblocks_test.go | Render order/content are partially validated; explicit marker framing/order assertions are still missing (`KIT-HEADELS-ISSUE-001`). | partial | kit/headels |
| KIT-HEADELS-010 | KHS-008 | kit-headels | go_package_unit | source+legacy-tests | kit/headels/headblocks_test.go | Error path for invalid meta element is verified; title/rest error wrappers remain source-only (`KIT-HEADELS-ISSUE-004`). | partial | kit/headels |
| KIT-HEADELS-011 | KHS-011 | kit-headels | go_package_unit | source+legacy-tests | kit/headels/headblocks_test.go | `New` behavior is exercised; `FromRaw` alias semantics remain source-defined (`KIT-HEADELS-ISSUE-002`). | partial | kit/headels |
| KIT-HEADELS-012 | KHS-009 | kit-headels | go_package_unit | source+legacy-tests | kit/headels/headblocks_test.go | `Add` type mapping, including known-safe attributes, booleans, self-closing, and inner-HTML/text content, is validated. | covered | kit/headels |
| KIT-HEADELS-013 | KHS-009 | kit-headels | go_package_unit | source+legacy-tests | kit/headels/headblocks_test.go | Missing-tag panic behavior is directly validated. | covered | kit/headels |
| KIT-HEADELS-014 | KHS-011 | kit-headels | go_package_unit | source+legacy-tests | kit/headels/headblocks_test.go | `Collect` clone semantics are directly tested; full `AddElements` copy/ordering details are partially source-derived (`KIT-HEADELS-ISSUE-002`). | partial | kit/headels |
| KIT-HEADELS-015 | KHS-012 | kit-headels | go_package_unit | source+legacy-tests | kit/headels/headblocks_test.go | Concurrent `Add` and `AddElements` usage is validated via parallel tests. | covered | kit/headels |
| KIT-HEADELS-016 | KHS-010 | kit-headels | go_package_unit | source+legacy-tests | kit/headels/headblocks_test.go | Several helper expansions are covered; remaining exported helper wrappers are source-only and issue-backed (`KIT-HEADELS-ISSUE-002`). | partial | kit/headels |
