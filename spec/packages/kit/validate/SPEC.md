# kit/validate Specification

Status: Active  
Last Updated: 2026-02-09  
Owner: `kit/validate`

## Scope

Package-owned contracts for decoding/parsing request input and running recursive
validation through fluent checkers and `Validator` implementations.

## Terminology

- "High-level helpers" means `JSONBodyInto`, `JSONBytesInto`, `JSONStrInto`, and
  `URLSearchParamsInto`.
- "Checker done state" means `AnyChecker.done == true`, which short-circuits
  later checks.
- "Recursive validation" means `validateRecursive` traversal over
  structs/maps/slices plus `Validator` hooks.

## Requirements

- `KIT-VALIDATE-001` `ValidationError` wrapper behavior.
  `ValidationError.Error()` MUST return inner error string and
  `ValidationError.Unwrap()` MUST expose wrapped error.
- `KIT-VALIDATE-002` Validation-error type detection behavior.
  `IsValidationError` MUST detect direct and wrapped `*ValidationError` values
  via `errors.As`.
- `KIT-VALIDATE-003` Destination nil-guard behavior. Shared destination
  validation MUST reject nil destinations.
- `KIT-VALIDATE-004` `JSONBodyInto` request guards. `JSONBodyInto` MUST return
  validation errors for nil request and nil request body.
- `KIT-VALIDATE-005` `JSONBodyInto` decode behavior. `JSONBodyInto` MUST decode
  JSON body into destination and wrap decode failures as validation errors.
- `KIT-VALIDATE-006` `JSONBodyInto` validation behavior. After successful
  decode, `JSONBodyInto` MUST run `attemptValidation` and propagate validation
  failures.
- `KIT-VALIDATE-007` `JSONBytesInto` decode+validate behavior. `JSONBytesInto`
  MUST reject nil destination, decode JSON bytes, and run `attemptValidation`.
- `KIT-VALIDATE-008` `JSONStrInto` decode+validate behavior. `JSONStrInto` MUST
  reject nil destination, decode JSON string, and run `attemptValidation`.
- `KIT-VALIDATE-009` `URLSearchParamsInto` request guards. `URLSearchParamsInto`
  MUST return validation errors for nil request and nil request URL.
- `KIT-VALIDATE-010` `URLSearchParamsInto` parse+validate behavior.
  `URLSearchParamsInto` MUST reject nil destination, parse query values via
  `parseURLValues`, wrap parse failures, and run `attemptValidation`.
- `KIT-VALIDATE-011` `parseURLValues` destination contract. `parseURLValues`
  MUST require destination to be a non-nil pointer to struct.
- `KIT-VALIDATE-012` `parseURLValues` interface-pointer behavior. When
  destination pointer points at an interface, parsing MUST dereference interface
  value before struct-kind checks.
- `KIT-VALIDATE-013` Field-name mapping and embedding behavior. URL parsing MUST
  use JSON field names (`reflectutil.GetJSONFieldName`) and recursively process
  anonymous embedded fields.
- `KIT-VALIDATE-014` Nested struct URL parsing behavior. Nested struct fields
  MUST be populated from dotted prefixes (`parent.child`).
- `KIT-VALIDATE-015` Nested map URL parsing behavior. Map fields MUST be
  populated from dotted prefixes and delegated through `setMapField`.
- `KIT-VALIDATE-016` Slice URL parsing behavior. Slice fields MUST gather
  repeated keys, drop empty-string entries, and default to empty slice when no
  non-empty values exist.
- `KIT-VALIDATE-017` Pointer field assignment behavior. For pointer fields,
  empty-string input MUST set nil pointer; non-empty input MUST allocate and
  assign parsed value.
- `KIT-VALIDATE-018` Field dispatch behavior. `setField` MUST dispatch
  pointer/slice/map/scalar assignments and fail on unsupported field kinds.
- `KIT-VALIDATE-019` Scalar parse behavior. `setSingleValueField` MUST parse
  string/int/uint/float/bool values, and for non-pointer targets MUST treat
  empty string as no-op.
- `KIT-VALIDATE-020` Map and slice helper behavior. `setMapField` MUST
  initialize nil maps and support nested map recursion; `setSliceField` MUST
  populate elements (including pointer elements).
- `KIT-VALIDATE-021` `AnyChecker` required/optional initialization behavior.
  `Required` MUST fail effective-zero inputs; `Optional` MUST pass
  effective-zero inputs and mark checker done.
- `KIT-VALIDATE-022` Checker short-circuit behavior. After checker enters done
  state, subsequent rule methods and conditional callbacks MUST not alter
  outcome.
- `KIT-VALIDATE-023` `AnyChecker.Error` aggregation behavior. `AnyChecker.Error`
  MUST join collected errors and wrap them as `ValidationError`.
- `KIT-VALIDATE-024` `Object` constructor domain behavior. `Object` MUST accept
  only non-nil struct values or map values with string keys (including pointer
  forms).
- `KIT-VALIDATE-025` Object field lookup validation behavior. Struct field
  validation MUST fail for unknown or unexported fields; map-backed object
  lookup MUST use string key names.
- `KIT-VALIDATE-026` Object error aggregation behavior. `ObjectChecker.Error`
  MUST aggregate parent and child checker errors and remain idempotent across
  repeated calls.
- `KIT-VALIDATE-027` Recursive validator invocation behavior.
  `validateRecursive` MUST invoke `Validator.Validate()` on eligible values and
  preserve existing validation errors while prefixing non-validation errors with
  label context.
- `KIT-VALIDATE-028` Pointer-receiver validator behavior. Recursive validation
  MUST support pointer-receiver validators for addressable non-pointer values.
- `KIT-VALIDATE-029` Recursive traversal behavior. Recursive validation MUST
  walk exported struct fields, map keys and values, and slice/array elements
  with path-aware labels.
- `KIT-VALIDATE-030` `attemptValidation` entry behavior. `attemptValidation`
  MUST return nil for nil input; otherwise it MUST return `ValidationError` when
  recursive validation yields errors.
- `KIT-VALIDATE-031` `attemptValidation` pointer-copy behavior. When required
  for pointer-receiver validation on non-pointer values, `attemptValidation`
  MUST validate through a pointer copy.
- `KIT-VALIDATE-032` Utility zero/type-state behavior. `safeDereference`,
  `getTypeState`, `isEffectivelyZero`, and `safeIsNil` MUST classify
  pointer/interface/map/slice/struct state exactly per source implementation.
- `KIT-VALIDATE-033` Conditional rule behavior. `AnyChecker.If` MUST execute
  callback only when condition is true and checker is not done.
- `KIT-VALIDATE-034` Membership rule behavior. `In` and `NotIn` MUST validate
  against slice/array inputs, supporting pointer/custom-type value comparisons
  and failing on nil/empty/non-slice candidate sets.
- `KIT-VALIDATE-035` Field-group relationship rule behavior. `MutuallyExclusive`
  and `MutuallyRequired` MUST enforce field-group constraints using truthy field
  counting.
- `KIT-VALIDATE-036` String rule behavior. `PermittedChars`, `Email`, `Regex`,
  `StartsWith`, `EndsWith`, and `URL` MUST validate string-like values and
  report failures per source-defined messages/guards.
- `KIT-VALIDATE-037` Numeric/length rule behavior. `Min`, `Max`,
  `RangeInclusive`, and `RangeExclusive` MUST operate on numeric values and on
  lengths of string/slice/array/map values, and fail unsupported kinds.
- `KIT-VALIDATE-038` Validation-chain failure-stop behavior. After a failing
  rule marks checker done, later chained rules MUST not override the prior
  failure state.
- `KIT-VALIDATE-039` Multi-level error accumulation behavior. Validation over
  nested structs/maps/slices MUST accumulate multiple errors across levels
  rather than returning only first failure.
- `KIT-VALIDATE-040` Error labeling behavior. Nested validation errors MUST
  retain root/context label information in reported error strings.
- `KIT-VALIDATE-041` Optional object-field tolerance behavior. Optional fields
  in object checks MUST tolerate nil/zero values without introducing
  required-field failures.
- `KIT-VALIDATE-042` High-level helper error unification behavior. High-level
  helpers MUST surface parse and validation failures through
  `ValidationError`-detectable errors.
- `KIT-VALIDATE-043` URL parsing complex-structure behavior. URL search
  parameter parsing MUST support tested complex structures (nested structs,
  maps, slices, pointer fields, embedded structs, and mixed pointer/non-pointer
  fields).
- `KIT-VALIDATE-044` URL parsing conversion-failure behavior. URL parsing MUST
  fail on unsupported destination/field types and on scalar conversion errors.
