package stringsutil

import (
	"bufio"
	"fmt"
	"strings"
)

func CollectLines(s string) ([]string, error) {
	if len(s) == 0 {
		return nil, nil
	}
	scanner := bufio.NewScanner(strings.NewReader(s))
	// Raise the scanner token limit to handle long logical lines.
	scanner.Buffer(make([]byte, 0, 64*1024), len(s)+1)
	lines := make([]string, 0)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading lines: %w", err)
	}
	return lines, nil
}
