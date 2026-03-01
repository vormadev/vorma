# kit/lockfile

`github.com/vormadev/vorma/kit/lockfile`

File-based lease lock helper for cross-process mutual exclusion with stale-lock
takeover.

## Import

```go
import "github.com/vormadev/vorma/kit/lockfile"
```

## Quick Start

```go
lock := lockfile.NewPIDLock("./.wave-dev.lock")

if err := lock.Acquire(); err != nil {
	return err
}
defer lock.Release()

// exclusive work
```

## Custom Options

```go
lock := lockfile.NewPIDLockWithOptions(
	"./.wave-dev.lock",
	lockfile.Options{
		AcquireRetryLimit:            20,
		AcquireRetryDelay:            25 * time.Millisecond,
		InvalidPIDLockStaleThreshold: 2 * time.Second,
		LeaseHeartbeatInterval:       300 * time.Millisecond,
		LeaseStaleThreshold:          4 * time.Second,
	},
)
```

## Behavior Notes

- Lock files store lease JSON with schema version, owner id, pid, and heartbeat
  timestamp.
- `Acquire` uses atomic create (`O_EXCL`) and retries briefly on stale-lock
  remove races.
- Owners heartbeat while held; callers can take over when heartbeat is stale.
- Invalid/non-parseable lock contents are treated as held until
  `InvalidPIDLockStaleThreshold`, then reclaimed.
- `Release` is ownership-fenced and will not delete another owner’s lock file.
- `OnLeaseLost` can be used by long-running processes to fail fast when
  ownership is lost.

## API Coverage

- `var ErrLockHeld error`
- `type Options struct {`
- `	HeldError error`
- `	AcquireRetryLimit int`
- `	AcquireRetryDelay time.Duration`
- `	InvalidPIDLockStaleThreshold time.Duration`
- `	FileWriteMode fs.FileMode`
- `	DirectoryWriteMode fs.FileMode`
- `	ProcessAppearsAlive func(processID int)` (optional PID liveness override;
  defaults to a signal-0 probe)
- `	LeaseHeartbeatInterval time.Duration`
- `	LeaseStaleThreshold time.Duration`
- `	OnLeaseLost func()`
- `}`
- `type PIDLock struct`
- `func NewPIDLock(lockFilePath string) *PIDLock`
- `func NewPIDLockWithOptions(lockFilePath string, options Options) *PIDLock`
- `func (pidLock *PIDLock) Path() string`
- `func (pidLock *PIDLock) Acquire() error`
- `func (pidLock *PIDLock) Release() error`
- `func (pidLock *PIDLock) Held() bool`
