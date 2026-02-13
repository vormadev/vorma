package wave

import (
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/vormadev/vorma/kit/fsutil"
)

func (w *Wave) initFileMap() (FileMap, error) {
	base, err := w.GetBaseFS()
	if err != nil {
		return nil, err
	}

	f, err := base.Open(RelPaths.PublicFileMapGob())
	if err != nil {
		return nil, fmt.Errorf("open file map: %w", err)
	}
	defer f.Close()

	fm, err := fsutil.FromGob[FileMap](f)
	if err != nil {
		return nil, fmt.Errorf("decode file map: %w", err)
	}

	return fm, nil
}

func (w *Wave) GetPublicFileMap() (FileMap, error) {
	return w.fileMap.get()
}

func (w *Wave) resolvePublicURL(original string) (string, error) {
	if isPassthroughPublicURL(original) {
		return original, nil
	}

	fm, err := w.GetPublicFileMap()
	if err != nil {
		w.log.Warn("failed to load file map", "error", err)
	}

	url, found := fm.Lookup(original, w.cfg.PublicPathPrefix())
	if !found {
		w.log.Warn("no hashed URL found", "url", original)
	}

	return url, nil
}

func isPassthroughPublicURL(original string) bool {
	lowerOriginal := strings.ToLower(original)

	return strings.HasPrefix(lowerOriginal, "data:") ||
		strings.HasPrefix(lowerOriginal, "http://") ||
		strings.HasPrefix(lowerOriginal, "https://") ||
		strings.HasPrefix(lowerOriginal, "ws://") ||
		strings.HasPrefix(lowerOriginal, "wss://") ||
		strings.HasPrefix(lowerOriginal, "blob:") ||
		strings.HasPrefix(lowerOriginal, "file:") ||
		strings.HasPrefix(original, "//")
}

func (w *Wave) GetPublicURL(original string) string {
	url, _ := w.publicURLs.get(original)
	return url
}

func (w *Wave) checkIsAsset(urlPath string) (bool, error) {
	publicAssetPath, isPublicAssetPath := w.publicAssetPath(urlPath)
	if !isPublicAssetPath {
		return false, nil
	}

	publicFS, err := w.GetPublicFS()
	if err != nil {
		return false, err
	}

	info, err := fs.Stat(publicFS, publicAssetPath)
	if err != nil {
		return false, nil
	}

	return !info.IsDir(), nil
}

func (w *Wave) publicAssetPath(urlPath string) (string, bool) {
	cleanURLPath := path.Clean("/" + urlPath)
	if cleanURLPath == "/" {
		return "", false
	}

	prefix := w.cfg.PublicPathPrefix()
	if prefix != "/" {
		if !strings.HasPrefix(cleanURLPath, prefix) {
			return "", false
		}
		cleanURLPath = strings.TrimPrefix(cleanURLPath, prefix)
	}

	assetPath := strings.TrimPrefix(cleanURLPath, "/")
	if assetPath == "" || assetPath == "." {
		return "", false
	}

	return assetPath, true
}

func (w *Wave) IsPublicAsset(urlPath string) bool {
	isAsset, _ := w.isAsset.get(urlPath)
	return isAsset
}
