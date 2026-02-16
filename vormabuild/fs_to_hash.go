package vormabuild

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io/fs"
	"sort"
)

type fsFileSummary struct {
	path string
	size int64
}

func getFSSummaryHash(fsys fs.FS) ([]byte, error) {
	var fileSummaries []fsFileSummary
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
		fileSummaries = append(fileSummaries, fsFileSummary{
			path: p,
			size: info.Size(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(fileSummaries, func(i int, j int) bool {
		return fileSummaries[i].path < fileSummaries[j].path
	})

	hash := sha256.New()
	for _, fileSummary := range fileSummaries {
		hash.Write([]byte(fileSummary.path))
		hash.Write([]byte{0})

		sizeBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(sizeBytes, uint64(fileSummary.size))
		hash.Write(sizeBytes)
		hash.Write([]byte{0})

		fileContents, err := fs.ReadFile(fsys, fileSummary.path)
		if err != nil {
			return nil, fmt.Errorf("read %s for FS summary hash: %w", fileSummary.path, err)
		}
		hash.Write(fileContents)
		hash.Write([]byte{0})
	}
	return hash.Sum(nil), nil
}
