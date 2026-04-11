package stringutil

import (
	"strings"
	"testing"
)

func TestCollectLines_EmptyString(t *testing.T) {
	lines, err := CollectLines("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lines != nil {
		t.Fatalf("expected nil lines for empty input, got %v", lines)
	}
}

func TestCollectLines_MultipleLines(t *testing.T) {
	lines, err := CollectLines("a\nb\nc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "a" || lines[1] != "b" || lines[2] != "c" {
		t.Fatalf("unexpected lines: %v", lines)
	}
}

func TestCollectLines_LongSingleLine(t *testing.T) {
	input := strings.Repeat("x", 70*1024)

	lines, err := CollectLines(input)
	if err != nil {
		t.Fatalf("expected long-line input to succeed, got error: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0] != input {
		t.Fatalf("expected output line to match input")
	}
}
