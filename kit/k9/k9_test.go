package k9

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"
)

var k9Pattern = regexp.MustCompile(fmt.Sprintf(`^[a-z2-7]{%d}$`, stringSize))

func TestNew(t *testing.T) {
	id, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if len(id) != IDSize {
		t.Fatalf("len(New()) = %d, want %d", len(id), IDSize)
	}

	ts := id.ToUnixMicro()
	if ts <= 0 {
		t.Fatalf("ToUnixMicro(New()) = %d, want > 0", ts)
	}
	if ts%timestampResolutionMicros != 0 {
		t.Fatalf("ToUnixMicro(New()) = %d, want multiple of %d", ts, timestampResolutionMicros)
	}
	now := time.Now().UnixMicro()
	if delta := now - ts; delta < -5_000_000 || delta > 5_000_000 {
		t.Fatalf("New() timestamp outside expected range: now=%d id=%d delta=%d", now, ts, delta)
	}
}

func TestNewReturnsRandomReadError(t *testing.T) {
	originalReadRandom := readRandom
	t.Cleanup(func() {
		readRandom = originalReadRandom
	})

	expectedError := errors.New("random read failed")
	readRandom = func([]byte) (int, error) {
		return 0, expectedError
	}

	_, err := New()
	if !errors.Is(err, expectedError) {
		t.Fatalf("New() error = %v, want %v", err, expectedError)
	}
}

func TestNewMulti(t *testing.T) {
	ids, err := NewMulti(8)
	if err != nil {
		t.Fatalf("NewMulti(8) error = %v", err)
	}
	if len(ids) != 8 {
		t.Fatalf("len(NewMulti(8)) = %d, want 8", len(ids))
	}

	var expectedTimestamp int64
	for i, id := range ids {
		if len(id) != IDSize {
			t.Fatalf("len(NewMulti(8)[%d]) = %d, want %d", i, len(id), IDSize)
		}
		serialized := id.Serialize()
		if !k9Pattern.MatchString(serialized) {
			t.Fatalf(
				"NewMulti(8)[%d].Serialize() format = %q, want lowercase base32 (%d chars)",
				i,
				serialized,
				stringSize,
			)
		}
		ts := id.ToUnixMicro()
		if i == 0 {
			expectedTimestamp = ts
		}
		if ts != expectedTimestamp {
			t.Fatalf(
				"NewMulti(8) should share one timestamp tick, ids[%d]=%d want %d",
				i,
				ts,
				expectedTimestamp,
			)
		}
	}

	original := ids[1][0]
	ids[0][0] ^= 0xff
	if ids[1][0] != original {
		t.Fatalf("NewMulti(8) returned aliased IDs: mutating ids[0] changed ids[1]")
	}
}

func TestNewMultiReturnsRandomReadError(t *testing.T) {
	originalReadRandom := readRandom
	t.Cleanup(func() {
		readRandom = originalReadRandom
	})

	expectedError := errors.New("random read failed")
	readRandom = func([]byte) (int, error) {
		return 0, expectedError
	}

	_, err := NewMulti(2)
	if !errors.Is(err, expectedError) {
		t.Fatalf("NewMulti(2) error = %v, want %v", err, expectedError)
	}
}

func TestNewMultiZero(t *testing.T) {
	ids, err := NewMulti(0)
	if err != nil {
		t.Fatalf("NewMulti(0) error = %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("len(NewMulti(0)) = %d, want 0", len(ids))
	}
}

func TestNewMultiNegative(t *testing.T) {
	_, err := NewMulti(-1)
	if err == nil {
		t.Fatalf("NewMulti(-1) expected error, got nil")
	}
	if !errors.Is(err, ErrInvalidCount) {
		t.Fatalf("NewMulti(-1) error = %v, want ErrInvalidCount", err)
	}
}

func TestNewMultiCountTooLarge(t *testing.T) {
	tooLargeCount := int(^uint(0)>>1)/randomBytesSize + 1

	_, err := NewMulti(tooLargeCount)
	if err == nil {
		t.Fatalf("NewMulti(%d) expected error, got nil", tooLargeCount)
	}
	if !errors.Is(err, ErrInvalidCount) {
		t.Fatalf("NewMulti(%d) error = %v, want ErrInvalidCount", tooLargeCount, err)
	}
}

func TestMinIDAtTimeSortability(t *testing.T) {
	t0 := time.UnixMicro(1738838400123400)
	t1 := t0.Add(100 * time.Microsecond)

	id0 := MinIDAtTime(t0)
	id1 := MinIDAtTime(t1)

	if got := bytes.Compare(id0[:], id1[:]); got >= 0 {
		t.Fatalf("bytes.Compare(MinIDAtTime(t0), MinIDAtTime(t1)) = %d, want < 0", got)
	}
	if got := ToUnixMicro(id0); got != t0.UnixMicro() {
		t.Fatalf("ToUnixMicro(MinIDAtTime(t0)) = %d, want %d", got, t0.UnixMicro())
	}
	if got := CreationTime(id1); !got.Equal(t1) {
		t.Fatalf("CreationTime(MinIDAtTime(t1)) = %s, want %s", got, t1)
	}
}

func TestMinIDAtTimeRoundsDownToResolution(t *testing.T) {
	in := time.UnixMicro(1738838400123456)
	id := MinIDAtTime(in)
	want := int64(1738838400123400)

	if got := id.ToUnixMicro(); got != want {
		t.Fatalf("ToUnixMicro(MinIDAtTime(%d)) = %d, want %d", in.UnixMicro(), got, want)
	}
}

func TestMinIDAtTimeClampsToSupportedRange(t *testing.T) {
	maxUnixMicro := int64(maxTimestampTicks) * timestampResolutionMicros
	tooLarge := time.UnixMicro(maxUnixMicro + timestampResolutionMicros*100)

	if got := MinIDAtTime(tooLarge).ToUnixMicro(); got != maxUnixMicro {
		t.Fatalf("ToUnixMicro(MinIDAtTime(tooLarge)) = %d, want %d", got, maxUnixMicro)
	}
	if got := MinIDAtTime(time.UnixMicro(-1)).ToUnixMicro(); got != 0 {
		t.Fatalf("ToUnixMicro(MinIDAtTime(pre-epoch)) = %d, want 0", got)
	}
}

func TestMaxIDAtTimeForGivenTime(t *testing.T) {
	tick := time.UnixMicro(1738838400123400)
	lower := MinIDAtTime(tick)
	upper := MaxIDAtTime(tick)

	if lower.ToUnixMicro() != upper.ToUnixMicro() {
		t.Fatalf("MaxIDAtTime timestamp = %d, want %d", upper.ToUnixMicro(), lower.ToUnixMicro())
	}
	if bytes.Compare(lower[:], upper[:]) > 0 {
		t.Fatalf("bytes.Compare(lower, upper) should be <= 0")
	}
	if upper[6]&0x0f != 0x0f {
		t.Fatalf("upper low nibble at byte 6 = %x, want f", upper[6]&0x0f)
	}
	for i := 7; i < IDSize; i++ {
		if upper[i] != 0xff {
			t.Fatalf("upper[%d] = %x, want ff", i, upper[i])
		}
	}
}

func TestMinAndMaxIDAtTimeEncloseTickIDs(t *testing.T) {
	tick := time.UnixMicro(1738838400123400)
	lower := MinIDAtTime(tick)
	upper := MaxIDAtTime(tick)

	id := lower
	id[6] |= 0x0a
	id[7] = 0x01
	id[15] = 0xfe

	if bytes.Compare(id[:], lower[:]) < 0 {
		t.Fatalf("id should be >= lower bound")
	}
	if bytes.Compare(id[:], upper[:]) > 0 {
		t.Fatalf("id should be <= upper bound")
	}
}

func TestSerializeDoesNotMutateAndIsDeterministic(t *testing.T) {
	id := ID{
		0x00, 0x01, 0x02, 0x03,
		0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b,
		0x0c, 0x0d, 0x0e, 0x0f,
	}
	original := id

	first := Serialize(id)
	second := id.Serialize()

	if id != original {
		t.Fatalf("Serialize() mutated ID: got=%x want=%x", id[:], original[:])
	}
	if first != second {
		t.Fatalf("Serialize() function/method mismatch: function=%q method=%q", first, second)
	}
	if !k9Pattern.MatchString(first) {
		t.Fatalf("Serialize() format = %q, want lowercase base32 (%d chars)", first, stringSize)
	}
}

func TestRoundTripForGeneratedIDs(t *testing.T) {
	for i := 0; i < 256; i++ {
		id, err := New()
		if err != nil {
			t.Fatalf("New() error at iteration %d: %v", i, err)
		}

		encoded := id.Serialize()
		if !k9Pattern.MatchString(encoded) {
			t.Fatalf(
				"Serialize() format at iteration %d = %q, want lowercase base32 (%d chars)",
				i,
				encoded,
				stringSize,
			)
		}

		decoded, err := Parse(encoded)
		if err != nil {
			t.Fatalf("Parse(Serialize(id)) error at iteration %d: %v", i, err)
		}
		if decoded != id {
			t.Fatalf(
				"Parse(Serialize(id)) mismatch at iteration %d: got=%x want=%x",
				i,
				decoded[:],
				id[:],
			)
		}

		reEncoded := decoded.Serialize()
		if reEncoded != encoded {
			t.Fatalf(
				"Serialize(Parse(Serialize(id))) mismatch at iteration %d: got=%q want=%q",
				i,
				reEncoded,
				encoded,
			)
		}
	}
}

func TestParseIsCaseInsensitive(t *testing.T) {
	id := ID{
		0x10, 0x20, 0x30, 0x40,
		0x50, 0x60, 0x70, 0x80,
		0x90, 0xa0, 0xb0, 0xc0,
		0xd0, 0xe0, 0xf0, 0x11,
	}
	encoded := id.Serialize()

	upperDecoded, err := Parse(strings.ToUpper(encoded))
	if err != nil {
		t.Fatalf("Parse(strings.ToUpper(encoded)) error = %v", err)
	}
	lowerDecoded, err := Parse(encoded)
	if err != nil {
		t.Fatalf("Parse(encoded) error = %v", err)
	}
	if upperDecoded != lowerDecoded {
		t.Fatalf(
			"Parse() should decode uppercase/lowercase the same: upper=%x lower=%x",
			upperDecoded[:],
			lowerDecoded[:],
		)
	}
}

func TestParseRejectsInvalidLength(t *testing.T) {
	tests := []string{
		"",
		"aa",
		strings.Repeat("a", stringSize-1),
		strings.Repeat("a", stringSize+1),
	}

	for _, tc := range tests {
		_, err := Parse(tc)
		if err == nil {
			t.Fatalf("Parse(%q) expected error, got nil", tc)
		}
		if !errors.Is(err, ErrInvalidTextLength) {
			t.Fatalf("Parse(%q) error = %v, want ErrInvalidTextLength", tc, err)
		}
	}
}

func TestParseRejectsInvalidCharacters(t *testing.T) {
	invalid := strings.Repeat("a", stringSize-1) + "!"
	_, err := Parse(invalid)
	if err == nil {
		t.Fatalf("Parse(%q) expected base32 decode error, got nil", invalid)
	}
}

func TestFromBytesValidAndCopiesInput(t *testing.T) {
	in := []byte{
		0x00, 0x01, 0x02, 0x03,
		0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b,
		0x0c, 0x0d, 0x0e, 0x0f,
	}
	id, err := FromBytes(in)
	if err != nil {
		t.Fatalf("FromBytes(valid) error = %v", err)
	}
	if !bytes.Equal(id[:], in) {
		t.Fatalf("FromBytes(valid) mismatch: got=%x want=%x", id[:], in)
	}

	in[0] ^= 0xff
	if bytes.Equal(id[:], in) {
		t.Fatalf("FromBytes should copy input: mutating source changed ID")
	}
}

func TestFromBytesRejectsInvalidLength(t *testing.T) {
	tests := [][]byte{
		nil,
		{},
		{1, 2, 3},
		make([]byte, IDSize-1),
		make([]byte, IDSize+1),
	}
	for _, tc := range tests {
		_, err := FromBytes(tc)
		if err == nil {
			t.Fatalf("FromBytes(len=%d) expected error, got nil", len(tc))
		}
		if !errors.Is(err, ErrInvalidIDLength) {
			t.Fatalf("FromBytes(len=%d) error = %v, want ErrInvalidIDLength", len(tc), err)
		}
	}
}

func TestToUnixMicroZeroID(t *testing.T) {
	if got := ToUnixMicro(ID{}); got != 0 {
		t.Fatalf("ToUnixMicro(zero ID) = %d, want 0", got)
	}
}

func TestIDCreationTimeMethodMatchesFunction(t *testing.T) {
	id := MinIDAtTime(time.UnixMicro(1738838400123400))
	got := id.CreationTime()
	want := CreationTime(id)
	if !got.Equal(want) {
		t.Fatalf("id.CreationTime() = %s, want %s", got, want)
	}
}
