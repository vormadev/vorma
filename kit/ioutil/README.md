# kit/ioutil

`github.com/vormadev/vorma/kit/ioutil`

Read helpers for bounded stream reads.

## Import

```go
import "github.com/vormadev/vorma/kit/ioutil"
```

## `ReadLimited`

```go
data, err := ioutil.ReadLimited(r, 5*ioutil.OneMB)
if errors.Is(err, ioutil.ErrReadLimitExceeded) {
	// too large; data is truncated to exactly the limit
	return err
}
if err != nil {
	return err
}
```

Behavior:

- Reads up to `limit + 1` bytes internally to detect overflow.
- Returns full data with `nil` error when input size is `<= limit`.
- Returns truncated data (`limit` bytes) with `ErrReadLimitExceeded` when input exceeds the limit.

## Limits and Bounds

- `limit` is `uint64`.
- Extremely large limits above `math.MaxInt64-1` return an internal limit-too-large error.
- This guard exists because the underlying `io.LimitReader` takes `int64`.

## Constants

- `OneKB`
- `OneMB`
- `OneGB`

## API Reference

- `const OneKB uint64`
- `const OneMB`
- `const OneGB`
- `var ErrReadLimitExceeded`
- `func ReadLimited(r io.Reader, limit uint64) ([]byte, error)`
