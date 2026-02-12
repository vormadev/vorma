package wave

import (
	"io/fs"
	"path"
	"strings"

	"github.com/vormadev/vorma/kit/matcher"
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

func joinPublicURLFromRefPath(publicPathPrefix string, refPath string) string {
	if refPath == "" {
		return ""
	}

	return matcher.EnsureLeadingSlash(path.Join(publicPathPrefix, refPath))
}

func (w *Wave) initPublicURLFromInternalRefFile(relativePath string) (string, error) {
	refPath, err := w.readTrimmedInternalRefFile(relativePath)
	if err != nil {
		return "", err
	}

	return joinPublicURLFromRefPath(w.cfg.PublicPathPrefix(), refPath), nil
}
