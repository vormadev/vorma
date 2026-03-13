package wave4

import (
	"context"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/kit/strict"
)

type super_state struct {
	watch_root strict.CWDRelPath
}

type SuperOptions struct {
	ParentContext         context.Context
	CWDRelativeConfigPath strict.CWDRelPath
}

func Super(opts SuperOptions) {
	ctx_to_use := opts.ParentContext
	config_path := opts.CWDRelativeConfigPath
	_ = config_path

	if ctx_to_use == nil {
		ctx_to_use = context.Background()
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic("wave: supervisor: create watcher: " + err.Error())
	}
	defer watcher.Close()
}

func watch(watcher *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(
		root,
		func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if err := watcher.Add(path); err != nil {
					return err
				}
			}
			return nil
		},
	)
}
