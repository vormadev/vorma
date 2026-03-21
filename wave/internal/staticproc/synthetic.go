package staticproc

import (
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"
)

// ToSyntheticFS builds an fs.FS that presents the original logical
// source paths, while resolving all reads to the flat, hashed files
// in flat_fs. Directory structure is synthesized from the filemap
// keys. Both file and directory opens are O(1) map lookups.
//
// Returns an error if the filemap contains a path that is both a
// file and a directory prefix (e.g. "a" and "a/b").
func ToSyntheticFS(flat_fs fs.FS, filemap map[string]string) (fs.FS, error) {
	files := make(map[string]string, len(filemap))
	dir_children := map[string][]string{".": nil}

	for logical, resolved := range filemap {
		logical = strings.TrimPrefix(logical, "/")
		resolved = strings.TrimPrefix(resolved, "/")
		if logical == "" {
			continue
		}
		files[logical] = resolved

		parts := strings.Split(logical, "/")
		for i := range parts {
			var parent string
			if i == 0 {
				parent = "."
			} else {
				parent = strings.Join(parts[:i], "/")
			}
			dir_children[parent] = append(dir_children[parent], parts[i])
		}
	}

	// Reject file/dir collisions: a logical path cannot be both a
	// file and a directory prefix.
	for logical := range files {
		if _, is_dir := dir_children[logical]; is_dir {
			return nil, fmt.Errorf(
				"file/dir collision: %q exists as both a file and a directory prefix",
				logical,
			)
		}
	}

	dirs := make(map[string][]fs.DirEntry, len(dir_children))
	for dir_path, children := range dir_children {
		sort.Strings(children)
		var entries []fs.DirEntry
		prev := ""
		for _, name := range children {
			if name == prev {
				continue
			}
			prev = name
			child_path := name
			if dir_path != "." {
				child_path = dir_path + "/" + name
			}
			_, is_dir := dir_children[child_path]
			if is_dir {
				entries = append(entries, &synth_dir_entry{
					info: &synth_dir_info{name_str: name},
				})
			} else {
				resolved := files[child_path]
				real_info, err := fs.Stat(flat_fs, resolved)
				if err != nil {
					return nil, fmt.Errorf(
						"stat %q (resolved %q): %w",
						child_path, resolved, err,
					)
				}
				entries = append(entries, &synth_dir_entry{
					info: &synth_file_info{
						FileInfo: real_info,
						name_str: name,
					},
				})
			}
		}
		dirs[dir_path] = entries
	}

	return &synthetic_fs{underlying: flat_fs, files: files, dirs: dirs}, nil
}

type synthetic_fs struct {
	underlying fs.FS
	files      map[string]string
	dirs       map[string][]fs.DirEntry
}

// ReadDir implements fs.ReadDirFS. Entries are pre-sorted by
// construction so no additional sort is needed.
func (m *synthetic_fs) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}
	entries, ok := m.dirs[name]
	if !ok {
		return nil, &fs.PathError{
			Op:   "readdir",
			Path: name,
			Err:  fs.ErrNotExist,
		}
	}
	// Return a copy to prevent callers from mutating our slice.
	result := make([]fs.DirEntry, len(entries))
	copy(result, entries)
	return result, nil
}

// Sub implements fs.SubFS by returning a new synthetic_fs scoped
// to the given directory.
func (m *synthetic_fs) Sub(dir string) (fs.FS, error) {
	if !fs.ValidPath(dir) {
		return nil, &fs.PathError{Op: "sub", Path: dir, Err: fs.ErrInvalid}
	}
	if dir == "." {
		return m, nil
	}
	if _, ok := m.dirs[dir]; !ok {
		return nil, &fs.PathError{Op: "sub", Path: dir, Err: fs.ErrNotExist}
	}

	prefix := dir + "/"
	new_files := make(map[string]string)
	for k, v := range m.files {
		if strings.HasPrefix(k, prefix) {
			new_files[strings.TrimPrefix(k, prefix)] = v
		}
	}
	new_dirs := make(map[string][]fs.DirEntry)
	new_dirs["."] = m.dirs[dir]
	for k, v := range m.dirs {
		if strings.HasPrefix(k, prefix) {
			new_dirs[strings.TrimPrefix(k, prefix)] = v
		}
	}

	return &synthetic_fs{
		underlying: m.underlying,
		files:      new_files,
		dirs:       new_dirs,
	}, nil
}

// Stat implements fs.StatFS, avoiding the fallback Open/Stat/Close
// round-trip. This is the hot path for http.FileServer.
func (m *synthetic_fs) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}
	if resolved, ok := m.files[name]; ok {
		info, err := fs.Stat(m.underlying, resolved)
		if err != nil {
			return nil, &fs.PathError{
				Op:   "stat",
				Path: name,
				Err:  fs.ErrNotExist,
			}
		}
		return &synth_file_info{FileInfo: info, name_str: path.Base(name)}, nil
	}
	if _, ok := m.dirs[name]; ok {
		dir_name := "."
		if name != "." {
			dir_name = path.Base(name)
		}
		return &synth_dir_info{name_str: dir_name}, nil
	}
	return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
}

// ReadFile implements fs.ReadFileFS, avoiding the synth_file wrapper
// overhead for bulk reads (e.g. loading templates from private FS).
func (m *synthetic_fs) ReadFile(name string) ([]byte, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrInvalid}
	}
	resolved, ok := m.files[name]
	if !ok {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrNotExist}
	}
	data, err := fs.ReadFile(m.underlying, resolved)
	if err != nil {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrNotExist}
	}
	return data, nil
}

func (m *synthetic_fs) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	if resolved, ok := m.files[name]; ok {
		f, err := m.underlying.Open(resolved)
		if err != nil {
			return nil, &fs.PathError{
				Op:   "open",
				Path: name,
				Err:  fs.ErrNotExist,
			}
		}
		return &synth_file{File: f, logical_name: path.Base(name)}, nil
	}

	if entries, ok := m.dirs[name]; ok {
		dir_name := "."
		if name != "." {
			dir_name = path.Base(name)
		}
		return &synth_open_dir{name_str: dir_name, entries: entries}, nil
	}

	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

/////////////////////////////////////////////////////////////////////
/////// synth_file
/////////////////////////////////////////////////////////////////////

type synth_file struct {
	fs.File
	logical_name string
}

func (f *synth_file) Stat() (fs.FileInfo, error) {
	info, err := f.File.Stat()
	if err != nil {
		return nil, err
	}
	return &synth_file_info{FileInfo: info, name_str: f.logical_name}, nil
}

func (f *synth_file) Seek(offset int64, whence int) (int64, error) {
	if s, ok := f.File.(io.Seeker); ok {
		return s.Seek(offset, whence)
	}
	return 0, &fs.PathError{
		Op:   "seek",
		Path: f.logical_name,
		Err:  fs.ErrInvalid,
	}
}

func (f *synth_file) ReadAt(b []byte, off int64) (int, error) {
	if ra, ok := f.File.(io.ReaderAt); ok {
		return ra.ReadAt(b, off)
	}
	return 0, &fs.PathError{
		Op:   "read",
		Path: f.logical_name,
		Err:  fs.ErrInvalid,
	}
}

/////////////////////////////////////////////////////////////////////
/////// synth_open_dir
/////////////////////////////////////////////////////////////////////

type synth_open_dir struct {
	name_str string
	entries  []fs.DirEntry
	offset   int
}

func (d *synth_open_dir) Stat() (fs.FileInfo, error) {
	return &synth_dir_info{name_str: d.name_str}, nil
}

func (d *synth_open_dir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.name_str, Err: fs.ErrInvalid}
}

func (d *synth_open_dir) Close() error { return nil }

func (d *synth_open_dir) ReadDir(n int) ([]fs.DirEntry, error) {
	if n <= 0 {
		rest := d.entries[d.offset:]
		d.offset = len(d.entries)
		return rest, nil
	}
	if d.offset >= len(d.entries) {
		return nil, io.EOF
	}
	end := min(d.offset+n, len(d.entries))
	result := d.entries[d.offset:end]
	d.offset = end
	if d.offset >= len(d.entries) {
		return result, io.EOF
	}
	return result, nil
}

/////////////////////////////////////////////////////////////////////
/////// metadata types
/////////////////////////////////////////////////////////////////////

type synth_file_info struct {
	fs.FileInfo
	name_str string
}

func (i *synth_file_info) Name() string { return i.name_str }

type synth_dir_info struct{ name_str string }

func (d *synth_dir_info) Name() string       { return d.name_str }
func (d *synth_dir_info) Size() int64        { return 0 }
func (d *synth_dir_info) Mode() fs.FileMode  { return fs.ModeDir | 0555 }
func (d *synth_dir_info) ModTime() time.Time { return time.Time{} }
func (d *synth_dir_info) IsDir() bool        { return true }
func (d *synth_dir_info) Sys() any           { return nil }

type synth_dir_entry struct {
	info fs.FileInfo
}

func (e *synth_dir_entry) Name() string { return e.info.Name() }
func (e *synth_dir_entry) IsDir() bool  { return e.info.IsDir() }

func (e *synth_dir_entry) Type() fs.FileMode          { return e.info.Mode().Type() }
func (e *synth_dir_entry) Info() (fs.FileInfo, error) { return e.info, nil }
