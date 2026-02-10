package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	allowedExt := map[string]bool{
		".md":   true,
		".yaml": true,
		".yml":  true,
		".tsv":  true,
		".json": true,
	}

	hits := make([]string, 0)
	err := filepath.WalkDir("spec", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !allowedExt[filepath.Ext(path)] {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		s := bufio.NewScanner(f)
		lineNo := 0
		for s.Scan() {
			lineNo++
			line := s.Text()
			if strings.Contains(line, "/Users/") {
				hits = append(hits, fmt.Sprintf("%s:%d:%s", path, lineNo, line))
			}
		}
		return s.Err()
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	if len(hits) > 0 {
		fmt.Fprintln(os.Stderr, "machine-specific absolute paths found under spec/:")
		for _, h := range hits {
			fmt.Fprintln(os.Stderr, h)
		}
		os.Exit(1)
	}
}
