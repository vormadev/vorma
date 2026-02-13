package tooling

import (
	"io/fs"
	"os"
	"path/filepath"
)

// AddDir adds a directory and its subdirectories to the watcher
func (w *Watcher) AddDir(root string) error {
	return filepath.WalkDir(root, func(path string, directoryEntry fs.DirEntry, err error) error {
		if err != nil || !directoryEntry.IsDir() {
			return err
		}

		if w.IsIgnoredDir(path) {
			return filepath.SkipDir
		}

		// Use absolute path as key to avoid duplicates
		absolutePath := w.norm(path)
		if _, exists := w.watchedDirs.Load(absolutePath); exists {
			return nil
		}

		if err := w.fsWatch.Add(path); err != nil {
			return err
		}

		w.watchedDirs.Store(absolutePath, true)
		return nil
	})
}

// RemoveStale removes watches for directories that no longer exist
func (w *Watcher) RemoveStale() {
	w.watchedDirs.Range(func(key, _ any) bool {
		path := key.(string)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			w.fsWatch.Remove(path)
			w.watchedDirs.Delete(path)
		}
		return true
	})
}
