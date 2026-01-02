package keyset

import (
	"errors"
	"fmt"
	"os"

	"github.com/vormadev/vorma/kit/bytesutil"
	"github.com/vormadev/vorma/kit/cryptoutil"
	"github.com/vormadev/vorma/kit/lazyget"
)

// Base64-encoded 32-byte root secret.
// To generate new root secrets, run `openssl rand -base64 32`.
type RootSecret = string

// Latest-first slice of base64-encoded 32-byte root secrets.
// To generate new root secrets, run `openssl rand -base64 32`.
type RootSecrets []RootSecret

// Latest-first slice of size 32 byte array pointers
type UnwrappedKeyset []cryptoutil.Key32

/////////////////////////////////////////////////////////////////////
/////// KEYSET WRAPPER
/////////////////////////////////////////////////////////////////////

type Keyset struct{ uks UnwrappedKeyset }

func FromUnwrapped(uks UnwrappedKeyset) (*Keyset, error) {
	ks := &Keyset{uks: uks}
	if err := ks.Validate(); err != nil {
		return nil, fmt.Errorf("error validating keyset: %w", err)
	}
	return ks, nil
}

func (wk *Keyset) Validate() error {
	if wk == nil {
		return fmt.Errorf("keyset is nil")
	}
	if len(wk.uks) == 0 {
		return fmt.Errorf("keyset is empty")
	}
	for i, key := range wk.uks {
		if key == nil {
			return fmt.Errorf("key %d in keyset is nil", i)
		}
		if len(key) != cryptoutil.KeySize {
			return fmt.Errorf("key %d in keyset is not 32 bytes", i)
		}
	}
	return nil
}

// Unwrap returns the underlying UnwrappedKeyset, which is a
// latest-first slice of size 32 byte array pointers.
func (wk *Keyset) Unwrap() UnwrappedKeyset { return wk.uks }

// First returns the first key in the keyset and returns an error
// if the keyset is nil or empty or if the first key is nil.
func (wk *Keyset) First() (cryptoutil.Key32, error) {
	if wk == nil {
		return nil, fmt.Errorf("keyset is nil")
	}
	if len(wk.uks) == 0 {
		return nil, fmt.Errorf("keyset is empty")
	}
	first := wk.uks[0]
	if first == nil {
		return nil, fmt.Errorf("first key in keyset is nil")
	}
	return first, nil
}

/////////////////////////////////////////////////////////////////////
/////// ATTEMPT
/////////////////////////////////////////////////////////////////////

// Attempt runs the provided function for each key in the keyset
// until either (i) an attempt does not return an error (meaning
// it succeeded) or (ii) all keys have been attempted. This is
// useful when you want to fallback to a prior key if the current
// key fails due to a recent rotation.
func Attempt[R any](ks *Keyset, f func(cryptoutil.Key32) (R, error)) (R, error) {
	var zeroR R
	uks := ks.Unwrap()
	if len(uks) == 0 {
		return zeroR, fmt.Errorf("keyset is empty")
	}
	var errs []error
	for i, k := range uks {
		if k == nil {
			return zeroR, fmt.Errorf("key %d is nil", i)
		}
		result, err := f(k)
		if err == nil {
			return result, nil
		}
		errs = append(errs, fmt.Errorf("key %d: %w", i, err))
	}
	return zeroR, errors.Join(errs...)
}

/////////////////////////////////////////////////////////////////////
/////// HKDF
/////////////////////////////////////////////////////////////////////

// Keyset.HKDF applies HKDF to each key in the base Keyset using the
// provided salt and info string, returning a new Keyset consisting
// of the derived keys.
func (ks *Keyset) HKDF(salt []byte, info string) (*Keyset, error) {
	uks := ks.Unwrap()
	if len(uks) == 0 {
		return nil, fmt.Errorf("root keyset is empty")
	}
	derivedKeys := make(UnwrappedKeyset, 0, len(uks))
	for i, rootKey := range uks {
		dk, err := cryptoutil.HkdfSha256(rootKey, salt, info)
		if err != nil {
			return nil, fmt.Errorf("error deriving key from root key %d: %w", i, err)
		}
		derivedKeys = append(derivedKeys, dk)
	}
	return &Keyset{uks: derivedKeys}, nil
}

// Pass in a latest-first slice of environment variable names pointing
// to base64-encoded 32-byte root secrets.
// Example: LoadRootKeyset("CURRENT_SECRET", "PREVIOUS_SECRET")
func LoadRootKeyset(latestFirstEnvVarNames ...string) (*Keyset, error) {
	rootSecrets, err := LoadRootSecrets(latestFirstEnvVarNames...)
	if err != nil {
		return nil, fmt.Errorf("error loading root secrets: %w", err)
	}
	keyset, err := RootSecretsToRootKeyset(rootSecrets)
	if err != nil {
		return nil, fmt.Errorf("error converting root secrets to keyset: %w", err)
	}
	return keyset, nil
}

// RootSecretsToRootKeyset converts a slice of base64-encoded root
// secrets into a Keyset.
func RootSecretsToRootKeyset(rootSecrets RootSecrets) (*Keyset, error) {
	if len(rootSecrets) == 0 {
		return nil, fmt.Errorf("at least 1 root secret is required")
	}
	keys := make(UnwrappedKeyset, 0, len(rootSecrets))
	for i, secret := range rootSecrets {
		secretBytes, err := bytesutil.FromBase64(secret)
		if err != nil {
			return nil, fmt.Errorf(
				"error decoding base64 secret %d: %w", i, err,
			)
		}
		if len(secretBytes) != cryptoutil.KeySize {
			return nil, fmt.Errorf("secret %d is not 32 bytes", i)
		}
		key32, err := cryptoutil.ToKey32(secretBytes)
		if err != nil {
			return nil, fmt.Errorf("error converting secret %d to Key32: %w", i, err)
		}
		keys = append(keys, key32)
	}
	return &Keyset{uks: keys}, nil
}

// Pass in a latest-first slice of environment variable names pointing
// to base64-encoded 32-byte root secrets.
// Example: LoadRootSecrets("CURRENT_SECRET", "PREVIOUS_SECRET")
func LoadRootSecrets(latestFirstEnvVarNames ...string) (RootSecrets, error) {
	if len(latestFirstEnvVarNames) == 0 {
		return nil, fmt.Errorf("at least 1 env var key is required")
	}
	rootSecrets := make(RootSecrets, 0, len(latestFirstEnvVarNames))
	for i, envVarName := range latestFirstEnvVarNames {
		if envVarName == "" {
			return nil, fmt.Errorf("env var name at index %d is empty", i)
		}
		secret := os.Getenv(envVarName)
		if secret == "" {
			return nil, fmt.Errorf("env var %s is not set", envVarName)
		}
		rootSecrets = append(rootSecrets, RootSecret(secret))
	}
	return rootSecrets, nil
}

/////////////////////////////////////////////////////////////////////
/////// APP KEYSET
/////////////////////////////////////////////////////////////////////

type AppKeysetConfig struct {
	// Provide a latest-first slice of environment variable names pointing
	// to base64-encoded 32-byte root secrets.
	// Example: []string{"CURRENT_SECRET", "PREVIOUS_SECRET"}
	LatestFirstEnvVarNames []string
	// Passed into the salt parameter of downstream HKDF functions.
	// Once set, do not change this unless you want and entirely new keyset.
	ApplicationName string
	// When instantiated via MustAppKeyset, if this is true, panics
	// due to misconfiguration are deferred to the first use of the
	// keyset rather than at instantiation time.
	DeferPanic bool
}

type AppKeyset struct {
	rootFn      func() *Keyset
	hkdfFnMaker func(purpose string) func() *Keyset
}

func (ak *AppKeyset) Root() *Keyset                      { return ak.rootFn() }
func (ak *AppKeyset) HKDF(purpose string) func() *Keyset { return ak.hkdfFnMaker(purpose) }

// Panics if anything is misconfigured. If desired, you can defer the panic to the first
// use (rather than at instantiation) by passing in a true boolean as the second argument.
func MustAppKeyset(cfg AppKeysetConfig) *AppKeyset {
	var validateOrPanic = func() {
		if len(cfg.LatestFirstEnvVarNames) == 0 {
			panic("at least 1 env var key is required for AppKeysetConfig.LatestFirstEnvVarNames")
		}
		if cfg.ApplicationName == "" {
			panic("AppKeysetConfig.ApplicationName cannot be empty")
		}
	}
	if !cfg.DeferPanic {
		validateOrPanic()
	}
	rootFn := lazyget.New(func() *Keyset {
		if cfg.DeferPanic {
			validateOrPanic()
		}
		rootKeyset, err := LoadRootKeyset(cfg.LatestFirstEnvVarNames...)
		if err != nil {
			panic(fmt.Sprintf("error loading root keyset: %v", err))
		}
		return rootKeyset
	})
	return &AppKeyset{
		rootFn: rootFn,
		hkdfFnMaker: func(purpose string) func() *Keyset {
			return lazyget.New(func() *Keyset {
				if cfg.DeferPanic {
					validateOrPanic()
				}
				if purpose == "" {
					panic("HKDF purpose cannot be empty")
				}
				derivedKeyset, err := rootFn().HKDF([]byte(cfg.ApplicationName), purpose)
				if err != nil {
					panic(fmt.Sprintf(
						"error deriving keyset for purpose '%s' with application name '%s': %v",
						purpose, cfg.ApplicationName, err,
					))
				}
				return derivedKeyset
			})
		},
	}
}
