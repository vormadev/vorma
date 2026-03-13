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
lock := lockfile.NewPIDLock("./my-lock.lock")
if err := lock.Acquire(); err != nil {
    return err
}
defer lock.Release()
// exclusive work
```

## Custom Options

```go
lock := lockfile.NewPIDLockWithOptions(
    "./my-lock.lock",
    lockfile.Options{
        LeaseHeartbeatInterval: 300 * time.Millisecond,
        LeaseStaleThreshold:    4 * time.Second,
        OnLeaseLost: func() {
            log.Fatal("lock ownership lost")
        },
    },
)
```

## Behavior Notes

- Lock files store lease JSON with schema version, owner id, pid, and heartbeat
  timestamp.
- `Acquire` uses atomic create (`O_EXCL`) and retries briefly on stale-lock
  remove races.
- Owners heartbeat while held; callers can take over when heartbeat is stale.
- Invalid or non-parseable lock contents are treated as held until an internal
  staleness threshold (1200ms) is exceeded, then reclaimed.
- `Release` is ownership-fenced and will not delete another owner's lock file.
- `OnLeaseLost` can be used by long-running processes to fail fast when
  ownership is lost.

## API Coverage

- `var ErrLockHeld error`
- `type Options struct {`
- `  HeldError error`
- `  LeaseHeartbeatInterval time.Duration`
- `  LeaseStaleThreshold time.Duration`
- `  OnLeaseLost func()`
- `}`
- `type PIDLock struct`
- `func NewPIDLock(lockFilePath string) *PIDLock`
- `func NewPIDLockWithOptions(lockFilePath string, options Options) *PIDLock`
- `func (lock *PIDLock) Path() string`
- `func (lock *PIDLock) Acquire() error`
- `func (lock *PIDLock) Release() error`
- `func (lock *PIDLock) Held() bool`
