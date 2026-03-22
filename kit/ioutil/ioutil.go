package ioutil

import (
	"errors"
	"io"
	"math"
)

const (
	OneKB uint64 = 1024
	OneMB        = 1024 * OneKB
	OneGB        = 1024 * OneMB
)

var ErrReadLimitExceeded = errors.New("read limit exceeded")
var errReadLimitTooLarge = errors.New("read limit too large")
var errReaderIsNil = errors.New("reader is nil")

// ReadLimited reads data from the provided reader up to the given limit.
// It returns ErrReadLimitExceeded if the data exceeds the specified limit.
// Under the hood, it reads a single extra byte to check if the limit is
// exceeded. If that is a concern for your use case, just use io.LimitReader
// directly.
func ReadLimited(r io.Reader, limit uint64) ([]byte, error) {
	if r == nil {
		return nil, errReaderIsNil
	}

	if limit > uint64(math.MaxInt64-1) {
		return nil, errReadLimitTooLarge
	}

	// Read one extra byte to allow checking if the limit is exceeded
	limitReader := io.LimitReader(r, int64(limit)+1)

	data, err := io.ReadAll(limitReader)
	if err != nil {
		return data, err
	}

	// Check if the limit was exceeded
	if uint64(len(data)) > limit {
		return data[:limit], ErrReadLimitExceeded
	}

	return data, nil
}
