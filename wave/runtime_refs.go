package wave

import (
	"io/fs"
	"strings"
)

func (w *Wave) readTrimmedInternalRefFile(relativePath string) (string, error) {
	baseFS, err := w.GetBaseFS()
	if err != nil {
		return "", err
	}

	content, err := fs.ReadFile(baseFS, relativePath)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(content)), nil
}

func (w *Wave) initPublicURLFromInternalRefFile(relativePath string) (string, error) {
	refPath, err := w.readTrimmedInternalRefFile(relativePath)
	if err != nil {
		return "", err
	}

	return ResolvePublicURLFromReferencedPath(w.cfg.PublicPathPrefix(), refPath), nil
}
