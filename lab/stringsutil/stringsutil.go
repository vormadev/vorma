package stringsutil

import (
	"bufio"
	"fmt"
	"strings"
)

type Builder struct {
	sb strings.Builder
}

func (b *Builder) Write(str string) *Builder {
	b.sb.WriteString(str)
	return b
}

func (b *Builder) Return() *Builder {
	return b.Write("\n")
}

func (b *Builder) Tab() *Builder {
	return b.Write("\t")
}

func (b *Builder) Writef(format string, a ...any) *Builder {
	return b.Write(fmt.Sprintf(format, a...))
}

func (b *Builder) Line(line string) *Builder {
	return b.Write(line).Return()
}

func (b *Builder) Linef(format string, a ...any) *Builder {
	return b.Writef(format, a...).Return()
}

func (b *Builder) Space() *Builder {
	return b.Write(" ")
}

func (b *Builder) String() string {
	return b.sb.String()
}

func CollectLines(s string) ([]string, error) {
	if len(s) == 0 {
		return nil, nil
	}
	scanner := bufio.NewScanner(strings.NewReader(s))
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
