# kit/keyset

`github.com/vormadev/vorma/kit/keyset`

`keyset` is a latest-first key rotation primitive for symmetric crypto
workflows.

Use it to:

- load root secrets from environment variables
- validate and access active/previous keys
- derive purpose-specific keysets with HKDF
- attempt verify/decrypt across rotated keys

## Import

```go
import "github.com/vormadev/vorma/kit/keyset"
```

## Core Concepts

- `RootSecret`: one base64-encoded 32-byte secret string
- `RootSecrets`: latest-first list of root secrets (`current`, then
  `previous`...)
- `UnwrappedKeyset`: latest-first list of `cryptoutil.Key32`
- `Keyset`: wrapper around `UnwrappedKeyset` with validation helpers

Ordering matters: index `0` is always the active key used for new writes.

## Quick Start (Env-Backed)

```go
root, err := keyset.LoadRootKeyset("APP_SECRET_CURRENT", "APP_SECRET_PREVIOUS")
if err != nil {
	return err
}

activeKey, err := root.First()
if err != nil {
	return err
}
_ = activeKey
```

## Rotation Fallback (`Attempt`)

Use `Attempt` for read paths where payloads may have been created with older
keys:

```go
value, err := keyset.Attempt(root, func(k cryptoutil.Key32) (string, error) {
	return decryptWithKey(k, ciphertext)
})
if err != nil {
	return err
}
_ = value
```

Behavior:

- stops at first success
- if all keys fail, returns `errors.Join(...)` of per-key failures
- returns error immediately for nil/empty keysets or nil key entries

## HKDF Derivation

Derive a parallel latest-first keyset for a specific purpose:

```go
cookieKeys, err := root.HKDF([]byte("my-app"), "cookies")
if err != nil {
	return err
}
_ = cookieKeys
```

Each root key is derived independently, preserving order.

## High-Level Lazy API (`AppKeyset`)

`MustAppKeyset` wraps env loading + HKDF in lazy getters:

```go
appKeys := keyset.MustAppKeyset(keyset.AppKeysetConfig{
	LatestFirstEnvVarNames: []string{"APP_SECRET_CURRENT", "APP_SECRET_PREVIOUS"},
	ApplicationName:        "my-app",
})

root := appKeys.Root()            // lazy load root keyset once
csrfKeys := appKeys.HKDF("csrf") // returns lazy getter
_ = csrfKeys()                    // derive once for this purpose
_ = root
```

Misconfiguration behavior:

- `MustAppKeyset` panics for invalid config
- `DeferPanic: true` shifts that panic from constructor-time to first use
- `AppKeyset.HKDF("")()` panics (empty purpose)

## API Contracts And Footguns

- `Keyset.Unwrap()` returns the underlying key slice directly (no defensive
  copy).
- Mutating the returned slice mutates the `Keyset` internals.
- `FromUnwrapped` validates input but does not clone it.
- `LoadRootSecrets` treats missing or empty env values as errors.

If you need immutability guarantees, copy before sharing.

## Public API Reference

### Type Aliases / Types

- `type RootSecret = string`
- `type RootSecrets []RootSecret`
- `type UnwrappedKeyset []cryptoutil.Key32`
- `type Keyset struct`
- `type AppKeysetConfig struct`
- `type AppKeyset struct`

### Exported Fields (`AppKeysetConfig`)

- `LatestFirstEnvVarNames []string`
- `ApplicationName string`
- `DeferPanic bool`

### Constructors / Functions

- `func FromUnwrapped(uks UnwrappedKeyset) (*Keyset, error)`
- `func Attempt[R any](ks *Keyset, f func(cryptoutil.Key32) (R, error)) (R, error)`
- `func LoadRootSecrets(latestFirstEnvVarNames ...string) (RootSecrets, error)`
- `func RootSecretsToRootKeyset(rootSecrets RootSecrets) (*Keyset, error)`
- `func LoadRootKeyset(latestFirstEnvVarNames ...string) (*Keyset, error)`
- `func MustAppKeyset(cfg AppKeysetConfig) *AppKeyset`

### Methods

- `func (wk *Keyset) Validate() error`
- `func (wk *Keyset) Unwrap() UnwrappedKeyset`
- `func (wk *Keyset) First() (cryptoutil.Key32, error)`
- `func (ks *Keyset) HKDF(salt []byte, info string) (*Keyset, error)`
- `func (ak *AppKeyset) Root() *Keyset`
- `func (ak *AppKeyset) HKDF(purpose string) func() *Keyset`
