package fsutil

import (
	"bytes"
	"encoding/gob"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"
)

// TestEnsureDir tests the EnsureDir function.
func TestEnsureDir(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "testdir")

	// Clean up before and after test
	defer os.RemoveAll(dir)

	// Ensure the directory does not exist
	os.RemoveAll(dir)

	// Test creating the directory
	err := EnsureDir(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Check if the directory was created
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Fatalf("expected directory to exist, got %v", err)
	}

	// Test when directory already exists
	err = EnsureDir(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// TestCopyFile tests the CopyFile function.
func TestCopyFile(t *testing.T) {
	srcFile := filepath.Join(os.TempDir(), "testfile.txt")
	dstFile := filepath.Join(os.TempDir(), "testfile_copy.txt")

	// Clean up before and after test
	defer os.Remove(srcFile)
	defer os.Remove(dstFile)

	// Create a source file
	content := []byte("Hello, World!")
	if err := os.WriteFile(srcFile, content, 0644); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Test copying the file
	err := CopyFile(srcFile, dstFile)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Check if the destination file exists
	if _, err := os.Stat(dstFile); os.IsNotExist(err) {
		t.Fatalf("expected destination file to exist, got %v", err)
	}

	// Check if the content is the same
	copiedContent, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("expected no error reading copied file, got %v", err)
	}
	if !bytes.Equal(content, copiedContent) {
		t.Fatalf("expected content %s, got %s", content, copiedContent)
	}

	// Ensure file mode is preserved.
	srcInfo, err := os.Stat(srcFile)
	if err != nil {
		t.Fatalf("expected no error statting source file, got %v", err)
	}
	dstInfo, err := os.Stat(dstFile)
	if err != nil {
		t.Fatalf("expected no error statting destination file, got %v", err)
	}
	if srcInfo.Mode().Perm() != dstInfo.Mode().Perm() {
		t.Fatalf(
			"expected destination mode %v, got %v",
			srcInfo.Mode().Perm(),
			dstInfo.Mode().Perm(),
		)
	}
}

func TestCopyFileCreatesDestinationDirectories(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "source.txt")
	dstFile := filepath.Join(dstDir, "nested", "path", "target.txt")

	if err := os.WriteFile(srcFile, []byte("hello"), 0o600); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	if err := CopyFile(srcFile, dstFile); err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}

	got, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("failed to read destination file: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("unexpected copied content: %q", string(got))
	}
}

func TestCopyFile_ReturnsErrorWhenSourceAndDestinationAreSameFile(
	t *testing.T,
) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "same-file.txt")
	originalContent := []byte("keep me")

	if err := os.WriteFile(filePath, originalContent, 0o600); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	err := CopyFile(filePath, filePath)
	if err == nil {
		t.Fatal("expected CopyFile to fail for same source/destination file")
	}

	currentContent, readErr := os.ReadFile(filePath)
	if readErr != nil {
		t.Fatalf("failed to read source file after CopyFile: %v", readErr)
	}
	if !bytes.Equal(currentContent, originalContent) {
		t.Fatalf(
			"CopyFile modified source file for same source/destination; got %q, want %q",
			string(currentContent),
			string(originalContent),
		)
	}
}

// TestCopyDir tests the CopyDir function.
func TestCopyDir(t *testing.T) {
	srcDir := filepath.Join(os.TempDir(), "testdir_src")
	dstDir := filepath.Join(os.TempDir(), "testdir_dst")

	// Clean up before and after test
	defer os.RemoveAll(srcDir)
	defer os.RemoveAll(dstDir)

	// Create source directory and files
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("expected no error creating source dir, got %v", err)
	}

	file1 := filepath.Join(srcDir, "file1.txt")
	file2 := filepath.Join(srcDir, "file2.txt")
	subDir := filepath.Join(srcDir, "subdir")
	subFile := filepath.Join(subDir, "file3.txt")

	if err := os.WriteFile(file1, []byte("File 1"), 0644); err != nil {
		t.Fatalf("expected no error writing file1, got %v", err)
	}
	if err := os.WriteFile(file2, []byte("File 2"), 0644); err != nil {
		t.Fatalf("expected no error writing file2, got %v", err)
	}
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("expected no error creating subdir, got %v", err)
	}
	if err := os.WriteFile(subFile, []byte("File 3"), 0644); err != nil {
		t.Fatalf("expected no error writing subFile, got %v", err)
	}

	// Test copying the directory
	err := CopyDir(srcDir, dstDir)
	if err != nil {
		t.Fatalf("expected no error copying directory, got %v", err)
	}

	// Check if the destination directory and files exist
	checkFileContent(t, filepath.Join(dstDir, "file1.txt"), "File 1")
	checkFileContent(t, filepath.Join(dstDir, "file2.txt"), "File 2")
	checkFileContent(t, filepath.Join(dstDir, "subdir", "file3.txt"), "File 3")
}

func TestCopyDirRejectsSymlinkEntries(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is environment-dependent on Windows")
	}

	srcDir := t.TempDir()
	dstDir := filepath.Join(t.TempDir(), "dst")

	targetPath := filepath.Join(srcDir, "target.txt")
	if err := os.WriteFile(targetPath, []byte("target"), 0o644); err != nil {
		t.Fatalf("expected no error writing target file, got %v", err)
	}

	linkPath := filepath.Join(srcDir, "target-link.txt")
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Skipf("symlink is unavailable in this environment: %v", err)
	}

	copyError := CopyDir(srcDir, dstDir)
	if copyError == nil {
		t.Fatal("expected CopyDir to fail when source contains symlink entries")
	}
	if !strings.Contains(
		copyError.Error(),
		"symlink entries are not supported",
	) {
		t.Fatalf("copy error=%v, expected symlink rejection message", copyError)
	}
}

// Helper function to check file content
func checkFileContent(t *testing.T, filePath, expectedContent string) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("expected no error reading %s, got %v", filePath, err)
	}
	if string(content) != expectedContent {
		t.Fatalf("expected content %s, got %s", expectedContent, content)
	}
}

// mockFile is a minimal implementation of fs.File for testing.
type mockFile struct {
	io.Reader
}

func (f *mockFile) Stat() (fs.FileInfo, error) {
	return nil, nil
}

func (f *mockFile) Close() error {
	return nil
}

// TestFromGobInto tests the FromGobInto function.
func TestFromGobInto(t *testing.T) {
	type TestStruct struct {
		Name string
		Age  int
	}
	srcStruct := TestStruct{Name: "Alice", Age: 30}

	// Create a gob file
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(&srcStruct); err != nil {
		t.Fatalf("expected no error encoding struct, got %v", err)
	}

	// Create a mock fs.File
	mockFile := &mockFile{Reader: &buf}

	// Test decoding into a destination struct
	var dstStruct TestStruct
	err := FromGobInto(mockFile, &dstStruct)
	if err != nil {
		t.Fatalf("expected no error decoding gob, got %v", err)
	}

	// Check if the decoded struct matches the source struct
	if srcStruct != dstStruct {
		t.Fatalf("expected %v, got %v", srcStruct, dstStruct)
	}

	// Test decoding with nil file
	err = FromGobInto(nil, &dstStruct)
	if err == nil {
		t.Fatalf("expected error for nil file, got nil")
	}

	// Test decoding with nil destination
	err = FromGobInto(mockFile, nil)
	if err == nil {
		t.Fatalf("expected error for nil destination, got nil")
	}
}

// TestFromGob tests the FromGob function.
func TestFromGob(t *testing.T) {
	type TestStruct struct {
		Name string
		Age  int
	}
	srcStruct := TestStruct{Name: "Alice", Age: 30}

	// Create a gob file
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(&srcStruct); err != nil {
		t.Fatalf("expected no error encoding struct, got %v", err)
	}

	// Create a mock fs.File
	mockFile := &mockFile{Reader: &buf}

	// Test decoding
	dstStruct, err := FromGob[TestStruct](mockFile)
	if err != nil {
		t.Fatalf("expected no error decoding gob, got %v", err)
	}

	// Check if the decoded struct matches the source struct
	if srcStruct != dstStruct {
		t.Fatalf("expected %v, got %v", srcStruct, dstStruct)
	}

	// Test decoding with nil file
	_, err = FromGob[TestStruct](nil)
	if err == nil {
		t.Fatalf("expected error for nil file, got nil")
	}
}

func TestMustSub_ReturnsSubFS(t *testing.T) {
	fileSystem := fstest.MapFS{
		"parent/child.txt": {
			Data: []byte("hello"),
		},
	}

	subFileSystem := MustSub(fileSystem, "parent")
	readBytes, err := fs.ReadFile(subFileSystem, "child.txt")
	if err != nil {
		t.Fatalf("failed to read file from MustSub result: %v", err)
	}
	if string(readBytes) != "hello" {
		t.Fatalf(
			"MustSub file content = %q, want %q",
			string(readBytes),
			"hello",
		)
	}
}

func TestMustSub_PanicsForInvalidSubdirectoryPath(t *testing.T) {
	defer func() {
		recoveredValue := recover()
		if recoveredValue == nil {
			t.Fatal("expected MustSub to panic for invalid subdirectory path")
		}
	}()

	_ = MustSub(fstest.MapFS{}, "..")
}

func TestMustReadFile_ReturnsFileContents(t *testing.T) {
	fileSystem := fstest.MapFS{
		"content.txt": {
			Data: []byte("value"),
		},
	}

	readBytes := MustReadFile(fileSystem, "content.txt")
	if string(readBytes) != "value" {
		t.Fatalf("MustReadFile() = %q, want %q", string(readBytes), "value")
	}
}

func TestMustReadFile_PanicsForMissingFile(t *testing.T) {
	defer func() {
		recoveredValue := recover()
		if recoveredValue == nil {
			t.Fatal("expected MustReadFile to panic for missing file")
		}
	}()

	_ = MustReadFile(fstest.MapFS{}, "missing.txt")
}
