package artifactio_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/vormadev/vorma/internal/artifactio"
)

func TestWriteAndCopyFileAtomically(t *testing.T) {
	root := t.TempDir()
	destinationPath := filepath.Join(root, "out", "payload.txt")
	content := []byte("atomic-content")
	if writeError := artifactio.WriteFileAtomically(destinationPath, content, 0o644); writeError != nil {
		t.Fatalf("WriteFileAtomically returned error: %v", writeError)
	}

	destinationBytes, readError := os.ReadFile(destinationPath)
	if readError != nil {
		t.Fatalf("read destination file: %v", readError)
	}
	if string(destinationBytes) != string(content) {
		t.Fatalf(
			"destination content = %q, want %q",
			string(destinationBytes),
			string(content),
		)
	}

	copyPath := filepath.Join(root, "out", "copy.txt")
	if copyError := artifactio.CopyFileAtomically(destinationPath, copyPath); copyError != nil {
		t.Fatalf("CopyFileAtomically returned error: %v", copyError)
	}
	copyBytes, readCopyError := os.ReadFile(copyPath)
	if readCopyError != nil {
		t.Fatalf("read copied file: %v", readCopyError)
	}
	if string(copyBytes) != string(content) {
		t.Fatalf(
			"copied file content = %q, want %q",
			string(copyBytes),
			string(content),
		)
	}
}

func TestWriteFileAtomically_ConcurrentWritersLeaveNoTemporaryArtifacts(
	t *testing.T,
) {
	root := t.TempDir()
	destinationPath := filepath.Join(root, "out", "payload.txt")

	var waitGroup sync.WaitGroup
	writeErrors := make(chan error, 64)
	for writerIndex := 0; writerIndex < 32; writerIndex++ {
		waitGroup.Add(1)
		go func(index int) {
			defer waitGroup.Done()
			content := []byte("writer-" + strconv.Itoa(index))
			if writeError := artifactio.WriteFileAtomically(
				destinationPath,
				content,
				0o644,
			); writeError != nil {
				writeErrors <- writeError
			}
		}(writerIndex)
	}
	waitGroup.Wait()
	close(writeErrors)

	for writeError := range writeErrors {
		t.Fatalf(
			"WriteFileAtomically returned error under concurrency: %v",
			writeError,
		)
	}

	destinationBytes, readError := os.ReadFile(destinationPath)
	if readError != nil {
		t.Fatalf("read destination file: %v", readError)
	}
	if !strings.HasPrefix(string(destinationBytes), "writer-") {
		t.Fatalf(
			"expected last-writer-wins content format, got %q",
			string(destinationBytes),
		)
	}

	temporaryArtifacts, globError := filepath.Glob(destinationPath + ".tmp-*")
	if globError != nil {
		t.Fatalf("glob temporary artifacts: %v", globError)
	}
	if len(temporaryArtifacts) != 0 {
		t.Fatalf(
			"expected no temporary artifacts after concurrent writes, found %#v",
			temporaryArtifacts,
		)
	}
}
