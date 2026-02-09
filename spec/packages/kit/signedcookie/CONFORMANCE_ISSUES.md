# kit/signedcookie Conformance Issues

Status: Active  
Last Updated: 2026-02-09

| Issue ID                     | Type                    | Affected Requirements                          | Summary                                                                                                                                                                                                                                                      | Status |
| ---------------------------- | ----------------------- | ---------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------ |
| `KIT-SIGNEDCOOKIE-ISSUE-001` | `intent-validation-gap` | `KIT-SIGNEDCOOKIE-006`, `KIT-SIGNEDCOOKIE-009` | Nil-input edge branches (`Manager.VerifyAndReadCookieValue(nil, ...)` and nil-receiver guards on `SignedCookie[T]` methods) are currently source-derived without direct legacy assertions. Confirm strict desired behavior and add targeted tests if needed. | open   |
