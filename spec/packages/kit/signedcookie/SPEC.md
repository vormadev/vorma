# kit/signedcookie Specification

Status: Active  
Last Updated: 2026-02-09  
Owner: `kit/signedcookie`

## Scope

Package-owned contracts for legacy signed-cookie helpers in
`kit/signedcookie/**`.

Current evidence note:

- Requirements are mined from `kit/signedcookie/signedcookie.go` and
  `kit/signedcookie/signedcookie_test.go`.
- This package is deprecated in favor of `kit/cookies`, but it remains normative
  for existing callers that still depend on this API/format.

## Requirements

- `KIT-SIGNEDCOOKIE-001` Manager initialization contract. `NewManager(secrets)`
  MUST convert root secrets through `keyset.RootSecretsToRootKeyset` and return
  a manager with initialized keyset; invalid/empty secrets MUST return errors.
- `KIT-SIGNEDCOOKIE-002` Manager initialization guard contract. Manager methods
  that rely on key material (`SignCookie`, `VerifyAndReadCookieValue`, internal
  sign/verify helpers) MUST fail when the manager keyset is uninitialized.
- `KIT-SIGNEDCOOKIE-003` Value signing contract.
  `signValue(unsignedValue, encrypt)` MUST: use `keyset.First()` as signer key,
  prefix payload with `0` (no encryption) or `1` (encrypted), sign via
  `cryptoutil.SignSymmetric`, and return base64 of `[prefix || signed-bytes]`.
- `KIT-SIGNEDCOOKIE-004` Value verification contract.
  `verifyAndReadValue(signedValue)` MUST: base64-decode input, require at least
  one prefix byte, reject unknown prefix bytes, verify signature via keyset
  attempt, decrypt when prefix is `1`, and return plaintext string.
- `KIT-SIGNEDCOOKIE-005` Key rotation read contract. Verification/read MUST
  attempt secrets in keyset order via `keyset.Attempt` so values signed with
  previous keys remain readable while those keys are present.
- `KIT-SIGNEDCOOKIE-006` Manager cookie-read API contract.
  `Manager.VerifyAndReadCookieValue(r, key)` MUST reject nil requests and empty
  keys, load named cookie from request, and return verified/decoded value.
- `KIT-SIGNEDCOOKIE-007` Manager cookie-sign API contract.
  `Manager.SignCookie(unsignedCookie, encrypt)` MUST reject nil cookie pointers
  and replace `unsignedCookie.Value` with signed value in place.
- `KIT-SIGNEDCOOKIE-008` Manager deletion-cookie contract.
  `Manager.NewDeletionCookie(cookie)` MUST return a cookie copy with `Value=""`
  and `MaxAge=-1` while preserving remaining attributes.
- `KIT-SIGNEDCOOKIE-009` Typed signed-cookie guard contract. `SignedCookie[T]`
  methods MUST enforce required fields: manager present and `BaseCookie.Name`
  non-empty; constructor/read methods also reject nil receiver.
- `KIT-SIGNEDCOOKIE-010` Typed unsigned-cookie synthesis contract.
  `newUnsignedCookie` MUST gob-encode typed value, base64-encode gob bytes, pick
  override base cookie when supplied (otherwise struct base cookie), set expiry
  to `time.Now().Add(TTL)` when `TTL != 0`, and create cookie via
  `newSecureCookieWithoutValue`.
- `KIT-SIGNEDCOOKIE-011` Secure base-cookie helper contract.
  `newSecureCookieWithoutValue(name, expires, baseCookie)` MUST copy optional
  base attributes, force cookie `Name`, conditionally set `Expires`, and always
  force `HttpOnly=true` and `Secure=true`.
- `KIT-SIGNEDCOOKIE-012` Typed sign/read roundtrip contract.
  `SignedCookie[T].NewSignedCookie` MUST sign generated cookie via manager and
  return signed cookie; `SignedCookie[T].VerifyAndReadCookieValue` MUST verify,
  base64-decode, gob-decode, and return typed value.
- `KIT-SIGNEDCOOKIE-013` Typed deletion-cookie contract.
  `SignedCookie[T].NewDeletionCookie` MUST return manager deletion cookie when
  receiver/configuration is valid, else return `nil`.
- `KIT-SIGNEDCOOKIE-014` Encryption parity contract. With identical typed
  payload, encrypted and unencrypted signing paths MUST produce different
  encoded cookie values while both decode back to equivalent typed values.
