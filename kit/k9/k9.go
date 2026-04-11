// Package k9 generates 16-byte k-sortable IDs, with human-friendly string encoding.
// Leading 52 bits are a Unix timestamp in 100-microsecond ticks (big endian).
// Trailing 76 bits are cryptographically random.
// String representation is 26 characters (base32-encoded, with no padding,
// and with bytes 0-6 XOR'd with bytes 7-13, respectively, to make it easier for
// human eyes to differentiate IDs).
// Only the byte representation is k-sortable, not the string representation.
package k9

import (
	"crypto/rand"
	"encoding/base32"
	"errors"
	"time"
)

const (
	// IDSize is the size, in bytes, of a k9 ID.
	IDSize = 16

	timeBytes       = 7
	randomBytesSize = 10
	stringSize      = 26
	timestampBits   = 52

	timestampResolutionMicros int64  = 100
	maxTimestampTicks         uint64 = (1 << timestampBits) - 1
)

var (
	b32Encoding = base32.StdEncoding.WithPadding(base32.NoPadding)
	readRandom  = rand.Read
	// ErrInvalidCount indicates an invalid count argument.
	ErrInvalidCount = errors.New("k9: invalid count")
	// ErrInvalidIDLength indicates an invalid raw ID byte length.
	ErrInvalidIDLength = errors.New("k9: invalid ID length")
	// ErrInvalidTextLength indicates an invalid encoded ID text length.
	ErrInvalidTextLength = errors.New("k9: invalid encoded ID length")
)

// ID is a 16-byte k9 ID.
type ID [IDSize]byte

// New generates a new 16-byte k9 ID, beginning with
// the unix timestamp at creation (in 100-microsecond ticks, big-endian),
// followed by 76 bits of cryptographic randomness.
func New() (ID, error) {
	var id ID
	var randomBytes10 [randomBytesSize]byte
	if _, err := readRandom(randomBytes10[:]); err != nil {
		return ID{}, err
	}
	// High nibble of byte 6 is reserved for timestamp bits.
	id[6] = randomBytes10[0] & 0x0f
	copy(id[7:], randomBytes10[1:])
	putTimestampTicks(&id, unixMicroToTimestampTicks(time.Now().UnixMicro()))
	return id, nil
}

// NewMulti generates n new IDs with unified error handling.
// Returns ErrInvalidCount when n is negative or too large.
// IDs produced by a single call share one timestamp tick.
func NewMulti(n int) ([]ID, error) {
	if n < 0 {
		return nil, ErrInvalidCount
	}
	maxCountByIDSlice := int(^uint(0)>>1) / IDSize
	maxCountByRandomBytes := int(^uint(0)>>1) / randomBytesSize
	if n > maxCountByIDSlice || n > maxCountByRandomBytes {
		return nil, ErrInvalidCount
	}

	ids := make([]ID, n)
	if n == 0 {
		return ids, nil
	}

	randomBytes := make([]byte, n*randomBytesSize)
	if _, err := readRandom(randomBytes); err != nil {
		return nil, err
	}

	timestampTicks := unixMicroToTimestampTicks(time.Now().UnixMicro())
	for i := range n {
		offset := i * randomBytesSize
		randomChunk := randomBytes[offset : offset+randomBytesSize]
		ids[i][6] = randomChunk[0] & 0x0f
		copy(ids[i][7:], randomChunk[1:])
		putTimestampTicks(&ids[i], timestampTicks)
	}

	return ids, nil
}

// ToUnixMicro converts a k9 ID to a Unix timestamp in microseconds.
func ToUnixMicro(id ID) int64 {
	return int64(readTimestampTicks(id)) * timestampResolutionMicros
}

// ToUnixMicro converts a k9 ID to a Unix timestamp in microseconds.
func (id ID) ToUnixMicro() int64 { return ToUnixMicro(id) }

// CreationTime converts a k9 ID to a time.Time object representing the
// creation time in microseconds.
func CreationTime(id ID) time.Time {
	return time.UnixMicro(ToUnixMicro(id))
}

// CreationTime converts a k9 ID to a time.Time object representing the
// creation time in microseconds.
func (id ID) CreationTime() time.Time { return CreationTime(id) }

// Serialize encodes a k9 ID into a 26-character, human-friendly
// string representation.
func Serialize(id ID) string {
	encoded := id
	for i := range timeBytes {
		encoded[i] ^= encoded[i+timeBytes]
	}
	var text [stringSize]byte
	b32Encoding.Encode(text[:], encoded[:])
	for i, c := range text {
		if c >= 'A' && c <= 'Z' {
			text[i] = c + ('a' - 'A')
		}
	}
	return string(text[:])
}

// Serialize encodes a k9 ID into a 26-character, human-friendly
// string representation.
func (id ID) Serialize() string { return Serialize(id) }

// MinIDAtTime returns the earliest possible ID for the given time.
func MinIDAtTime(t time.Time) ID {
	var id ID
	putTimestampTicks(&id, unixMicroToTimestampTicks(t.UnixMicro()))
	return id
}

// MaxIDAtTime returns the latest possible ID for the given time.
func MaxIDAtTime(t time.Time) ID {
	upper := MinIDAtTime(t)
	upper[6] |= 0x0f
	for i := 7; i < IDSize; i++ {
		upper[i] = 0xff
	}
	return upper
}

// Parse decodes a k9 ID string back into its byte representation.
func Parse(str string) (ID, error) {
	if len(str) != stringSize {
		return ID{}, ErrInvalidTextLength
	}

	var normalized [stringSize]byte
	for i := range stringSize {
		c := str[i]
		if c >= 'a' && c <= 'z' {
			c -= ('a' - 'A')
		}
		normalized[i] = c
	}

	var id ID
	if _, err := b32Encoding.Decode(id[:], normalized[:]); err != nil {
		return ID{}, err
	}
	for i := range timeBytes {
		id[i] ^= id[i+timeBytes]
	}
	return id, nil
}

// FromBytes validates and converts raw 16-byte input into a k9 ID.
func FromBytes(bytes []byte) (ID, error) {
	if len(bytes) != IDSize {
		return ID{}, ErrInvalidIDLength
	}
	var id ID
	copy(id[:], bytes)
	return id, nil
}

func unixMicroToTimestampTicks(unixMicro int64) uint64 {
	if unixMicro <= 0 {
		return 0
	}
	ticks := uint64(unixMicro / timestampResolutionMicros)
	if ticks > maxTimestampTicks {
		return maxTimestampTicks
	}
	return ticks
}

func putTimestampTicks(id *ID, ticks uint64) {
	id[0] = byte(ticks >> 44)
	id[1] = byte(ticks >> 36)
	id[2] = byte(ticks >> 28)
	id[3] = byte(ticks >> 20)
	id[4] = byte(ticks >> 12)
	id[5] = byte(ticks >> 4)
	id[6] = (id[6] & 0x0f) | (byte(ticks&0x0f) << 4)
}

func readTimestampTicks(id ID) uint64 {
	return (uint64(id[0]) << 44) |
		(uint64(id[1]) << 36) |
		(uint64(id[2]) << 28) |
		(uint64(id[3]) << 20) |
		(uint64(id[4]) << 12) |
		(uint64(id[5]) << 4) |
		uint64(id[6]>>4)
}
