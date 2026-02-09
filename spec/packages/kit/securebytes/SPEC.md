# kit/securebytes Specification

Status: Active  
Last Updated: 2026-02-09  
Owner: `kit/securebytes`

## Scope

Package-owned contracts for authenticated encrypted serialization in
`kit/securebytes/**`.

Current evidence note:

- Requirements are mined from `kit/securebytes/securebytes.go` and
  `kit/securebytes/securebytes_test.go`.
- `README.md` is guidance only and not treated as normative mining input.

## Requirements

- `KIT-SECUREBYTES-001` Serialize input-guard contract. `Serialize(ks, rv)` MUST
  reject `rv == nil` with `invalid raw value: nil value` and MUST reject invalid
  keysets via `ks.Validate()`.
- `KIT-SECUREBYTES-002` Serialize encoding/encryption contract. `Serialize` MUST
  gob-encode raw value, prepend package version byte, encrypt plaintext with
  `cryptoutil.EncryptSymmetricXChaCha20Poly1305` using `ks.First()` key, and
  return ciphertext as `SecureBytes`.
- `KIT-SECUREBYTES-003` Serialize max-size contract. `Serialize` MUST reject
  ciphertext larger than `MaxSize` (`1<<20`) with
  `ciphertext too large (over 1MB)`.
- `KIT-SECUREBYTES-004` Parse size/keyset guard contract. `Parse[T](ks, sb)`
  MUST reject empty `sb`, reject `len(sb) > MaxSize`, and reject invalid keysets
  via `ks.Validate()`.
- `KIT-SECUREBYTES-005` Parse decryption-attempt contract. `Parse` MUST attempt
  decryption across keyset entries via `keyset.Attempt` and return
  `error decrypting value: ...` on aggregate failure.
- `KIT-SECUREBYTES-006` Parse plaintext framing contract. After decrypt, `Parse`
  MUST require plaintext length >= 2 (version + gob payload), else fail with
  `invalid plaintext: too short`.
- `KIT-SECUREBYTES-007` Parse version contract. Decrypted version byte MUST
  equal `current_pkg_version` (`1`); mismatches MUST return
  `unsupported SecureBytes version <n>`.
- `KIT-SECUREBYTES-008` Parse gob-decoding contract. `Parse` MUST decode
  decrypted gob payload (`plaintext[1:]`) via `bytesutil.FromGob[T]` and return
  decoded typed value.
- `KIT-SECUREBYTES-009` Key-rotation compatibility contract. Values encrypted
  under prior keys MUST remain parseable when those keys remain present in
  keyset attempt order.
- `KIT-SECUREBYTES-010` Generic type roundtrip contract. Roundtrip semantics
  MUST preserve supported payload categories including primitives, structs, byte
  slices, pointer payloads, and nested composite values.
- `KIT-SECUREBYTES-011` Non-serializable payload contract. Serialize MUST fail
  when gob encoding unsupported values (for example channels and functions).
- `KIT-SECUREBYTES-012` Concurrency safety contract. Package APIs are stateless
  over caller-provided inputs and MUST support concurrent serialize/parse calls
  with shared valid keysets.
