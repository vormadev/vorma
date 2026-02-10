package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

var re = regexp.MustCompile("```(mermaid|plantuml|graphviz)")

func scanTree(root string, hits *[]string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
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
			if re.MatchString(s.Text()) {
				*hits = append(*hits, fmt.Sprintf("%s:%d:%s", path, lineNo, s.Text()))
			}
		}
		return s.Err()
	})
}

func main() {
	hits := make([]string, 0)
	for _, root := range []string{"spec/packages", "spec/_templates"} {
		if _, err := os.Stat(root); err != nil {
			continue
		}
		if err := scanTree(root, &hits); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	}
	if len(hits) > 0 {
		fmt.Fprintln(os.Stderr, "diagram/chart code blocks are prohibited in package artifacts and templates:")
		for _, h := range hits {
			fmt.Fprintln(os.Stderr, h)
		}
		os.Exit(1)
	}
}
