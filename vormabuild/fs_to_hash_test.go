package vormabuild

import (
	"bytes"
	"testing"
	"testing/fstest"
)

func TestGetFSSummaryHash_UsesPathAndSizeOnly(t *testing.T) {
	fsA := fstest.MapFS{
		"alpha.txt": {Data: []byte("ab")},
		"beta.txt":  {Data: []byte("xyz")},
	}
	fsB := fstest.MapFS{
		"alpha.txt": {Data: []byte("cd")},
		"beta.txt":  {Data: []byte("uvw")},
	}

	hashA, err := getFSSummaryHash(fsA)
	if err != nil {
		t.Fatalf("getFSSummaryHash(fsA) failed: %v", err)
	}
	hashB, err := getFSSummaryHash(fsB)
	if err != nil {
		t.Fatalf("getFSSummaryHash(fsB) failed: %v", err)
	}

	if !bytes.Equal(hashA, hashB) {
		t.Fatalf("expected equal hashes for same file paths/sizes, got %x and %x", hashA, hashB)
	}
}

func TestGetFSSummaryHash_ChangesWhenSummaryChanges(t *testing.T) {
	t.Run("size change changes hash", func(t *testing.T) {
		before := fstest.MapFS{
			"alpha.txt": {Data: []byte("ab")},
		}
		after := fstest.MapFS{
			"alpha.txt": {Data: []byte("abc")},
		}

		beforeHash, err := getFSSummaryHash(before)
		if err != nil {
			t.Fatalf("getFSSummaryHash(before) failed: %v", err)
		}
		afterHash, err := getFSSummaryHash(after)
		if err != nil {
			t.Fatalf("getFSSummaryHash(after) failed: %v", err)
		}

		if bytes.Equal(beforeHash, afterHash) {
			t.Fatalf("expected different hashes when size changes, got %x", beforeHash)
		}
	})

	t.Run("path change changes hash", func(t *testing.T) {
		before := fstest.MapFS{
			"alpha.txt": {Data: []byte("ab")},
		}
		after := fstest.MapFS{
			"beta.txt": {Data: []byte("ab")},
		}

		beforeHash, err := getFSSummaryHash(before)
		if err != nil {
			t.Fatalf("getFSSummaryHash(before) failed: %v", err)
		}
		afterHash, err := getFSSummaryHash(after)
		if err != nil {
			t.Fatalf("getFSSummaryHash(after) failed: %v", err)
		}

		if bytes.Equal(beforeHash, afterHash) {
			t.Fatalf("expected different hashes when path changes, got %x", beforeHash)
		}
	})
}
