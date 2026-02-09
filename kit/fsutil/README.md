# kit/fsutil

`github.com/vormadev/vorma/kit/fsutil`

Small filesystem helpers for common app/tooling tasks:

- ensure directories exist
- copy files/directories
- decode gob files
- fail-fast helpers for embedded filesystem access

## Import

```go
import "github.com/vormadev/vorma/kit/fsutil"
```

## Quick Start

### Ensure required directories

```go
if err := fsutil.EnsureDirs("var/cache", "var/log", "var/tmp"); err != nil {
	return err
}
```

### Copy a tree

```go
if err := fsutil.CopyDir("./templates", "./build/templates"); err != nil {
	return err
}
```

### Decode a gob file into a struct

```go
f, err := os.Open("state.gob")
if err != nil {
	return err
}
defer f.Close()

state, err := fsutil.FromGob[AppState](f)
if err != nil {
	return err
}
_ = state
```

## Behavior Notes

- `CopyFile` creates/overwrites destination content.
- `CopyFile` creates missing destination parent directories with mode `0755`.
- `CopyFile` preserves source file permission bits on destination file creation.
- `CopyDir` recursively copies directory entries and delegates file copies to
  `CopyFile`.
- `CopyDir`/`CopyFile` copy file contents and basic mode bits, not extended
  metadata.
- `MustSub` and `MustReadFile` panic on error; use them where failure is
  unrecoverable (for example required embedded assets).
- `GetCallerDir` returns the directory of the direct caller frame.

## API Coverage

### Functions

- `func CopyDir(src, dst string) error`
- `func CopyFile(src, dest string) error`
- `func CopyFiles(srcDestTuples ...[2]string) error`
- `func EnsureDir(path string) error`
- `func EnsureDirs(paths ...string) error`
- `func FromGob[T any](file fs.File) (T, error)`
- `func FromGobInto(file fs.File, destPtr any) error`
- `func GetCallerDir() string`
- `func MustReadFile(f fs.FS, name string) []byte`
- `func MustSub(f fs.FS, dirElems ...string) fs.FS`
