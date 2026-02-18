package wave

import (
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/vormadev/vorma/kit/fsutil"
)

func (w *Wave) initFileMap() (FileMap, error) {
	base, err := w.BaseFS()
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

func (w *Wave) PublicFileMap() (FileMap, error) {
	fileMap, err := w.fileMap.get()
	if err != nil {
		return nil, err
	}

	return clonePublicFileMap(fileMap), nil
}

func (w *Wave) resolvePublicURL(original string) (string, error) {
	fm, err := w.fileMap.get()
	if err != nil {
		w.log.Warn("failed to load file map", "error", err)
		return "", err
	}

	url, found := fm.Lookup(original, w.cfg.PublicPathPrefix())
	if !found {
		w.log.Warn("no hashed URL found", "url", original)
		return "", fmt.Errorf("no hashed URL found for %q", original)
	}

	return url, nil
}

func (w *Wave) PublicURL(original string) string {
	url, _ := w.publicURLs.get(original)
	return url
}

func (w *Wave) checkIsAsset(urlPath string) (bool, error) {
	publicAssetPath, isPublicAssetPath := w.publicAssetPath(urlPath)
	if !isPublicAssetPath {
		return false, nil
	}

	publicFS, err := w.PublicFS()
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

func clonePublicFileMap(
	fileMap FileMap,
) FileMap {
	if len(fileMap) == 0 {
		return nil
	}

	clonedPublicFileMap := make(FileMap, len(fileMap))
	for key, value := range fileMap {
		clonedPublicFileMap[key] = value
	}

	return clonedPublicFileMap
}
