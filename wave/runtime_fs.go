package wave

import (
	"fmt"
	"io/fs"
	"os"
)

func (w *Wave) initBaseFS() (fs.FS, error) {
	if w.IsDev() {
		return os.DirFS(w.cfg.Dist.Static()), nil
	}
	if w.distStaticFS == nil {
		return nil, fmt.Errorf("distStaticFS is nil in production mode")
	}
	return w.distStaticFS, nil
}

func (w *Wave) initPublicFS() (fs.FS, error) {
	return w.initSubFS(RelPaths.AssetsPublic())
}

func (w *Wave) initPrivateFS() (fs.FS, error) {
	return w.initSubFS(RelPaths.AssetsPrivate())
}

func (w *Wave) initSubFS(
	relativeSubdirectoryPath string,
) (fs.FS, error) {
	base, err := w.BaseFS()
	if err != nil {
		return nil, err
	}
	return fs.Sub(base, relativeSubdirectoryPath)
}

func (w *Wave) BaseFS() (fs.FS, error) {
	return w.baseFS.get()
}

func (w *Wave) PublicFS() (fs.FS, error) {
	return w.publicFS.get()
}

func (w *Wave) PrivateFS() (fs.FS, error) {
	return w.privateFS.get()
}

func (w *Wave) MustPublicFS() fs.FS {
	return mustGetCachedFileSystem(w.publicFS)
}

func (w *Wave) MustPrivateFS() fs.FS {
	return mustGetCachedFileSystem(w.privateFS)
}

func mustGetCachedFileSystem(
	cachedFileSystem *cache[fs.FS],
) fs.FS {
	fileSystem, err := cachedFileSystem.get()
	if err != nil {
		panic(err)
	}
	return fileSystem
}
