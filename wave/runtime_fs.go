package wave

import (
	"fmt"
	"io/fs"
	"os"
)

func (w *Wave) initBaseFS() (fs.FS, error) {
	if GetIsDev() {
		return os.DirFS(w.cfg.Dist.Static()), nil
	}
	if w.distStaticFS == nil {
		return nil, fmt.Errorf("distStaticFS is nil in production mode")
	}
	return w.distStaticFS, nil
}

func (w *Wave) initPublicFS() (fs.FS, error) {
	base, err := w.GetBaseFS()
	if err != nil {
		return nil, err
	}
	return fs.Sub(base, RelPaths.AssetsPublic())
}

func (w *Wave) initPrivateFS() (fs.FS, error) {
	base, err := w.GetBaseFS()
	if err != nil {
		return nil, err
	}
	return fs.Sub(base, RelPaths.AssetsPrivate())
}

func (w *Wave) GetBaseFS() (fs.FS, error) {
	return w.baseFS.get()
}

func (w *Wave) GetPublicFS() (fs.FS, error) {
	return w.publicFS.get()
}

func (w *Wave) GetPrivateFS() (fs.FS, error) {
	return w.privateFS.get()
}

func (w *Wave) MustGetPublicFS() fs.FS {
	f, err := w.publicFS.get()
	if err != nil {
		panic(err)
	}
	return f
}

func (w *Wave) MustGetPrivateFS() fs.FS {
	f, err := w.privateFS.get()
	if err != nil {
		panic(err)
	}
	return f
}
