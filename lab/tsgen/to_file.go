package tsgen

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/vormadev/vorma/kit/fsutil"
)

// GenerateTSToFile generates a TypeScript file from the provided Opts.
func GenerateTSToFile(opts Opts) error {
	if opts.OutPath == "" {
		return errors.New("outpath is required")
	}

	tsContent, err := GenerateTSContent(opts)
	if err != nil {
		return err
	}

	err = fsutil.EnsureDir(filepath.Dir(opts.OutPath))
	if err != nil {
		return errors.New("failed to ensure out dest dir: " + err.Error())
	}

	err = os.WriteFile(opts.OutPath, []byte(tsContent), os.ModePerm)
	if err != nil {
		return errors.New("failed to write ts file: " + err.Error())
	}

	return nil
}
