package builder

import (
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// hashFile computes a content-addressed filename.
// Includes originalName in the hash to prevent collisions when two files
// have identical content but different normalized names.
func hashFile(filePath, originalName string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()

	// Include original name to prevent collision edge case where:
	// 1. Two files have the same content
	// 2. Their underscore-normalized names would collide
	h.Write([]byte(originalName))

	buf := make([]byte, 32*1024)

	for {
		n, err := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}

	return formatHashedName(h, originalName), nil
}

// hashBytes computes a content-addressed filename for bytes
func hashBytes(content []byte, originalName string) string {
	h := sha256.New()
	h.Write([]byte(originalName)) // Include name to prevent collisions
	h.Write(content)
	return formatHashedName(h, originalName)
}

func formatHashedName(h hash.Hash, originalName string) string {
	hashStr := fmt.Sprintf("%x", h.Sum(nil))[:12]
	ext := filepath.Ext(originalName)
	base := strings.TrimSuffix(originalName, ext)
	return fmt.Sprintf("vorma_out_%s_%s%s", base, hashStr, ext)
}
