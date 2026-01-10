---
title: fsutil
description:
    Filesystem utilities for directory creation, file copying, and gob decoding.
---

```go
import "github.com/vormadev/vorma/kit/fsutil"
```

## Directory Operations

Create directory (and parents) if not exists:

```go
func EnsureDir(path string) error
func EnsureDirs(paths ...string) error
```

Get directory of the calling source file:

```go
func GetCallerDir() string
```

## Copying

```go
func CopyFile(src, dest string) error
func CopyFiles(srcDestTuples ...[2]string) error
func CopyDir(src, dst string) error
```

## Gob Decoding from File

Decode into pointer:

```go
func FromGobInto(file fs.File, destPtr any) error
```

Generic decode:

```go
func FromGob[T any](file fs.File) (T, error)
```

## Must Helpers (panic on error)

```go
func MustSub(f fs.FS, dirElems ...string) fs.FS
func MustReadFile(f fs.FS, name string) []byte
```
