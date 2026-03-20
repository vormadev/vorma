package staticproc

import (
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
func ToSyntheticFS(flat_fs fs.FS, filemap map[string]string) fs.FS {
	files := make(map[string]string, len(filemap))
	dir_children := map[string][]string{}

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
			entries = append(entries, &synth_dir_entry{
				name_str: name,
				is_dir:   is_dir,
			})
		}
		dirs[dir_path] = entries
	}

	return &synthetic_fs{underlying: flat_fs, files: files, dirs: dirs}
}

type synthetic_fs struct {
	underlying fs.FS
	files      map[string]string
	dirs       map[string][]fs.DirEntry
}

func (m *synthetic_fs) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	if resolved, ok := m.files[name]; ok {
		f, err := m.underlying.Open(resolved)
		if err != nil {
			return nil, &fs.PathError{Op: "open", Path: name, Err: err}
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
	name_str string
	is_dir   bool
}

func (e *synth_dir_entry) Name() string { return e.name_str }
func (e *synth_dir_entry) IsDir() bool  { return e.is_dir }
func (e *synth_dir_entry) Type() fs.FileMode {
	if e.is_dir {
		return fs.ModeDir
	}
	return 0
}
func (e *synth_dir_entry) Info() (fs.FileInfo, error) {
	if e.is_dir {
		return &synth_dir_info{name_str: e.name_str}, nil
	}
	return &synth_file_info{
		FileInfo: &synth_dir_info{name_str: e.name_str},
		name_str: e.name_str,
	}, nil
}
