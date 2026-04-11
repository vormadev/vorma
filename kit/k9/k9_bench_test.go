package k9

import (
	"strings"
	"testing"
)

var (
	benchSinkID     ID
	benchSinkString string
	benchSinkInt64  int64
)

func mustNewBenchmarkID(b *testing.B) ID {
	b.Helper()
	id, err := New()
	if err != nil {
		b.Fatalf("New() error: %v", err)
	}
	return id
}

func BenchmarkNew(b *testing.B) {
	for i := 0; i < b.N; i++ {
		id, err := New()
		if err != nil {
			b.Fatalf("New() error: %v", err)
		}
		benchSinkID = id
	}
}

func BenchmarkNewMulti16(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ids, err := NewMulti(16)
		if err != nil {
			b.Fatalf("NewMulti(16) error: %v", err)
		}
		benchSinkID = ids[0]
	}
}

func BenchmarkSerialize(b *testing.B) {
	id := mustNewBenchmarkID(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchSinkString = Serialize(id)
	}
}

func BenchmarkParseLower(b *testing.B) {
	id := mustNewBenchmarkID(b)
	s := id.Serialize()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parsed, err := Parse(s)
		if err != nil {
			b.Fatalf("Parse(lower) error: %v", err)
		}
		benchSinkID = parsed
	}
}

func BenchmarkParseUpper(b *testing.B) {
	id := mustNewBenchmarkID(b)
	s := strings.ToUpper(id.Serialize())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parsed, err := Parse(s)
		if err != nil {
			b.Fatalf("Parse(upper) error: %v", err)
		}
		benchSinkID = parsed
	}
}

func BenchmarkFromBytes(b *testing.B) {
	id := mustNewBenchmarkID(b)
	raw := id[:]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parsed, err := FromBytes(raw)
		if err != nil {
			b.Fatalf("FromBytes() error: %v", err)
		}
		benchSinkID = parsed
	}
}

func BenchmarkToUnixMicro(b *testing.B) {
	id := mustNewBenchmarkID(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchSinkInt64 = ToUnixMicro(id)
	}
}
