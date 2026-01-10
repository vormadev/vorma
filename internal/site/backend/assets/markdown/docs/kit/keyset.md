---
title: keyset
description:
    Cryptographic keyset management with rotation support and HKDF derivation.
---

```go
import "github.com/vormadev/vorma/kit/keyset"
```

## Types

```go
type RootSecret = string                      // base64-encoded 32-byte secret
type RootSecrets []RootSecret                 // latest-first
type UnwrappedKeyset []cryptoutil.Key32       // latest-first
```

## Keyset

```go
func FromUnwrapped(uks UnwrappedKeyset) (*Keyset, error)
func (ks *Keyset) Validate() error
func (ks *Keyset) Unwrap() UnwrappedKeyset
func (ks *Keyset) First() (cryptoutil.Key32, error)
func (ks *Keyset) HKDF(salt []byte, info string) (*Keyset, error)
```

## Key Rotation Helper

Try each key until success (useful for rotation fallback):

```go
func Attempt[R any](ks *Keyset, f func(cryptoutil.Key32) (R, error)) (R, error)
```

## Loading from Environment

```go
func LoadRootSecrets(latestFirstEnvVarNames ...string) (RootSecrets, error)
func LoadRootKeyset(latestFirstEnvVarNames ...string) (*Keyset, error)
func RootSecretsToRootKeyset(rootSecrets RootSecrets) (*Keyset, error)
```

## AppKeyset (High-Level)

Lazy-loaded keyset with HKDF derivation:

```go
type AppKeysetConfig struct {
    LatestFirstEnvVarNames []string
    ApplicationName        string  // used as HKDF salt
    DeferPanic             bool    // defer panic to first use
}

func MustAppKeyset(cfg AppKeysetConfig) *AppKeyset
func (ak *AppKeyset) Root() *Keyset
func (ak *AppKeyset) HKDF(purpose string) func() *Keyset
```

Example:

```go
var appKeys = keyset.MustAppKeyset(keyset.AppKeysetConfig{
    LatestFirstEnvVarNames: []string{"SECRET_CURRENT", "SECRET_PREVIOUS"},
    ApplicationName:        "myapp",
})

var encryptionKeys = appKeys.HKDF("encryption")

// Usage:
ks := encryptionKeys()
key, _ := ks.First()
```
