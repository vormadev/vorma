---
title: executil
description:
    Command execution utilities with stdout/stderr inherited from parent
    process.
---

```go
import "github.com/vormadev/vorma/kit/executil"
```

## Functions

Run command directly:

```go
func RunCmd(commands ...string) error
```

Run shell command (uses `sh -c` on Unix, `cmd /C` on Windows):

```go
func RunShell(command string) error
```

Create reusable command runner:

```go
func MakeCmdRunner(commands ...string) func() error
```

Get directory containing current executable:

```go
func GetExecutableDir() (string, error)
```

Example:

```go
executil.RunCmd("go", "build", "./...")
executil.RunShell("echo $HOME")
```
