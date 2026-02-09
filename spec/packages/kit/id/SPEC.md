# kit/id Specification

Status: Active  
Last Updated: 2026-02-09  
Owner: `kit/id`

## Scope

Package-owned cryptographically random ID generation contracts in `kit/id/**`.

Current evidence note:

- Requirements are mined from `kit/id/id.go` and `kit/id/id_test.go`.

## Requirements

- `KIT-ID-001` Default charset contract. `New` MUST use default mixed-case
  alphanumeric charset (`0-9A-Za-z`) when no custom charset is provided.
- `KIT-ID-002` Optional charset-arity contract. `New` MUST accept at most one
  optional charset argument and return
  `at most one optional charset may be provided` otherwise.
- `KIT-ID-003` Charset validation contract. Effective charset length MUST be
  between 1 and 255 inclusive; charset MUST contain only single-byte ASCII
  characters.
- `KIT-ID-004` Zero-length ID contract. `New(0, ...)` MUST return empty string
  after charset validation succeeds.
- `KIT-ID-005` Random ID generation contract. For `idLen > 0`, `New` MUST
  generate cryptographically random bytes via `crypto/rand.Read` and map
  accepted bytes into charset output.
- `KIT-ID-006` Modulo-bias mitigation contract. `New` MUST use rejection
  sampling with `effectiveTotalValues := (256/charsetLen)*charsetLen` to avoid
  modulo bias.
- `KIT-ID-007` Read-failure propagation contract. `New` MUST return
  `failed to read random bytes: ...` when `rand.Read` fails.
- `KIT-ID-008` Batch generation contract.
  `NewMulti(idLen, quantity, optionalCharset...)` MUST generate exactly
  `quantity` IDs by repeatedly calling `New`, preserving optional charset
  choice.
- `KIT-ID-009` Batch error propagation contract. `NewMulti` MUST return error
  immediately if any delegated `New` call fails.
- `KIT-ID-010` Output-shape contract. Returned IDs MUST be exactly requested
  lengths and contain only characters from the effective charset.
