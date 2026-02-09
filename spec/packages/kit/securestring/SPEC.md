# kit/securestring Specification

Status: Active  
Last Updated: 2026-02-09  
Owner: `kit/securestring`

## Scope

Package-owned contracts for base64 string wrappers around encrypted payloads in
`kit/securestring/**`.

Current evidence note:

- Requirements are mined from `kit/securestring/securestring.go` and
  `kit/securestring/securestring_test.go`.
- This package is a thin wrapper over `kit/securebytes`; owner semantics here
  are wrapper-specific contracts plus explicit delegation behavior.

## Requirements

- `KIT-SECURESTRING-001` Serialized form contract. `SecureString` values MUST
  represent base64-encoded ciphertext produced by `securebytes.Serialize`.
- `KIT-SECURESTRING-002` Serialize delegation contract. `Serialize(ks, rv)` MUST
  call `securebytes.Serialize(ks, rv)` and then base64-encode ciphertext via
  `bytesutil.ToBase64`.
- `KIT-SECURESTRING-003` Serialize error-wrapping contract. Errors from
  delegated `securebytes.Serialize` MUST be wrapped as
  `error serializing raw value: ...`.
- `KIT-SECURESTRING-004` Parse input-length guard contract. `Parse[T](ks, ss)`
  MUST reject empty `ss` with `invalid secure string: empty value` and reject
  `len(ss) > MaxBase64Size` with `secure string too large (over 1.33MB)`.
- `KIT-SECURESTRING-005` Parse base64 decoding contract. `Parse` MUST decode
  base64 ciphertext via `bytesutil.FromBase64`; decode failures MUST be wrapped
  as `error decoding base64: ...`.
- `KIT-SECURESTRING-006` Parse delegation contract. `Parse` MUST delegate
  decrypted parsing to
  `securebytes.Parse[T](ks, securebytes.SecureBytes(ciphertext))`.
- `KIT-SECURESTRING-007` Max size constant contract. `MaxBase64Size` MUST be
  computed from `securebytes.MaxSize` using
  `((securebytes.MaxSize + 2) / 3) * 4`.
- `KIT-SECURESTRING-008` Key-rotation compatibility contract. Values serialized
  under older keys MUST remain parseable when those keys are still present in
  keyset attempt order.
- `KIT-SECURESTRING-009` Generic payload roundtrip contract. Roundtrip behavior
  MUST preserve supported payload categories including primitives, structs,
  pointer payloads, and nested serializable values.
- `KIT-SECURESTRING-010` Invalid-input behavior contract. Parse MUST fail on
  invalid base64, tampered ciphertext, and keysets with no usable keys.
- `KIT-SECURESTRING-011` Version propagation contract. Version-byte validation
  is delegated to `securebytes.Parse`; unsupported version payloads MUST fail
  parsing.
- `KIT-SECURESTRING-012` Concurrency contract. Package APIs are stateless over
  caller inputs and MUST support concurrent serialize/parse calls with shared
  valid keysets.
