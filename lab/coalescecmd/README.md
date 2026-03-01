# lab/coalescecmd

`github.com/vormadev/vorma/lab/coalescecmd`

Function-level coalescing for expensive or mutating commands.

Use this inside the real command entrypoint (`main`/runner), so coalescing is
always enforced regardless of who invokes it.

## Import

```go
import "github.com/vormadev/vorma/lab/coalescecmd"
```

## Quick Start

```go
return coalescecmd.Run(coalescecmd.Options{
	Key:                    "buildts",
	StateRootDirectoryPath: "./internal/locks",
	Func: func() error {
		return runBuildTS()
	},
})
```

Panic-on-error variant:

```go
coalescecmd.MustRun(coalescecmd.Options{
	Key:                    "buildts",
	StateRootDirectoryPath: "./internal/locks",
	Func: func() error {
		return runBuildTS()
	},
})
```

## Behavior Notes

- Exactly one owner caller runs `Func` for a given key at a time.
- Concurrent callers with the same key wait for the owner and return the same
  final error (or `nil`).
- `StateRootDirectoryPath` is required; `Run` panics when omitted.
- Set `FailIfRunning` to a key list to fail immediately when any listed key is
  already running. Include your own `Key` when you want same-key callers to fail
  instead of waiting.
- Per key, artifacts are flat files under `StateRootDirectoryPath`:
  `<key-hash>.pid.lock` and `<key-hash>.state.json` (lowercase base32 hash, no
  padding).
- After a run completes, the next call starts a new run.
- Old completed run artifacts across all keys in `StateRootDirectoryPath` are
  cleaned up opportunistically after a short staleness window.

## API Coverage

- `const DefaultPollInterval = 50 * time.Millisecond`
- `var ErrAlreadyRunning error`
- `type Options struct {`
- `	Key string`
- `	Func func() error`
- `	FailIfRunning []string`
- `	StateRootDirectoryPath string`
- `	OwnerWaitPollingInterval time.Duration`
- `}`
- `func Run(options Options) error`
- `func MustRun(options Options)`
