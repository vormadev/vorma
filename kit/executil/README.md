# kit/executil

`github.com/vormadev/vorma/kit/executil`

Helpers for running subprocesses with stdout/stderr wired to the current
process.

## Import

```go
import "github.com/vormadev/vorma/kit/executil"
```

## Recommended Usage

Prefer `RunCmd`/`MakeCmdRunner` when possible:

```go
if err := executil.RunCmd("go", "test", "./..."); err != nil {
	return err
}
```

Reusable runner:

```go
build := executil.MakeCmdRunner("go", "build", "./cmd/server")
if err := build(); err != nil {
	return err
}
```

## Shell Usage

```go
// Unix: sh -c
// Windows: cmd /C
if err := executil.RunShell("echo hello"); err != nil {
	return err
}
```

Security note:

- `RunShell` executes via a shell and is vulnerable to shell injection if
  untrusted input is interpolated.
- Use `RunCmd` with explicit argument separation for user-controlled values.

## Executable Directory

```go
dir, err := executil.ExecutableDir()
```

## Behavior Notes

- `MakeCmdRunner` returns an error when called with no command.
- `RunCmd` and `RunShell` return command execution errors directly.
- Output streams are attached to `os.Stdout` and `os.Stderr`.

## API Reference

- `func MakeCmdRunner(commands ...string) func() error`
- `func RunCmd(commands ...string) error`
- `func RunShell(command string) error`
- `func ExecutableDir() (string, error)`
