package build

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
)

func getFSSummaryHash(fsys fs.FS) ([]byte, error) {
	var files []string
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == "." || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		files = append(files, fmt.Sprintf("%s|%d", p, info.Size()))
		return nil
	})
	if err != nil {
		return nil, err
	}
	hash := sha256.New()
	for _, file := range files {
		hash.Write([]byte(file))
	}
	return hash.Sum(nil), nil
}
