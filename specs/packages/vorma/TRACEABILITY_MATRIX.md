# vorma Traceability Matrix

Status: Draft  
Last Updated: 2026-02-09  
Applies To: Requirement-to-test traceability for `vorma` wrapper requirements

| Requirement ID | Scenario ID(s) | Suite Family | Suite Name | Fixture Type | Test File(s) | Pass Criteria | Status | Owner |
|---|---|---|---|---|---|---|---|---|
| VORMA-API-001 | VSC-API-001 | vorma-wrapper | go_wrapper_api_surface | go-fixture-app-http | conformance/backend/backend_init_conformance_test.go | Existing suite exercises root Go wrapper entrypoint usage but lacks dedicated wrapper-only compile/API gate. | partial | vorma |
| VORMA-API-002 | VSC-API-002 | vorma-wrapper | go_wrapper_loader_registration | source-contract | n/a (no dedicated wrapper helper conformance suite currently in repo) | Requirement remains missing until dedicated wrapper helper registration coverage is added. | missing | vorma |
| VORMA-API-003 | VSC-API-003 | vorma-wrapper | go_wrapper_action_registration | source-contract | n/a (no dedicated wrapper helper conformance suite currently in repo) | Requirement remains missing until dedicated wrapper helper registration coverage is added. | missing | vorma |
| VORMA-API-004 | VSC-API-004 | vorma-wrapper | go_wrapper_reexport_identity | source-contract | n/a (no dedicated wrapper re-export identity suite currently in repo) | Requirement remains missing until dedicated wrapper re-export identity coverage is added. | missing | vorma |
| VORMA-API-005 | VSC-API-005 | vorma-wrapper | go_wrapper_embedded_version | source-contract | n/a (no dedicated wrapper embedded-version suite currently in repo) | Requirement remains missing until dedicated wrapper embedded-version coverage is added. | missing | vorma |
