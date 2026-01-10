package dirs

import (
	"path/filepath"
	"reflect"
)

// NOTE! You must use exported (public, uppercased) fields
// or else this won't work. The build step for a directory
// structure uses reflection, so do it once when you start
// your app, and then cache it, so you don't have to do it
// over and over again.

type (
	empty = struct{}
	File  struct {
		name string
		path string
	}
	Dir[T any] struct {
		slash T
		name  string
		path  string
	}
	DirEmpty = Dir[empty]
	builder  interface {
		build(parent string)
	}
)

func (f *File) LastSegment() string { return f.name }
func (f *File) FullPath() string    { return f.path }

func (d *Dir[T]) LastSegment() string { return d.name }
func (d *Dir[T]) FullPath() string    { return d.path }
func (d *Dir[T]) S() T                { return d.slash }

func ToFile(name string) *File {
	return &File{name: name}
}
func ToRoot[T any](children T) *Dir[T] {
	return &Dir[T]{slash: children}
}
func ToDir[T any](name string, children T) *Dir[T] {
	return &Dir[T]{name: name, slash: children}
}
func ToDirEmpty(name string) *Dir[empty] {
	return &Dir[empty]{name: name}
}

func Build[T any](basePath string, root *Dir[T]) *Dir[T] {
	root.path = filepath.Join(basePath, root.name)
	reflectBuild(root.slash, root.path)
	return root
}

func (f *File) build(parent string) {
	f.path = filepath.Join(parent, f.name)
}

func (d *Dir[T]) build(parent string) {
	d.path = filepath.Join(parent, d.name)
	reflectBuild(d.slash, d.path)
}

func reflectBuild(data any, parentPath string) {
	if data == nil {
		return
	}
	val := reflect.ValueOf(data)

	if val.Kind() == reflect.Ptr && !val.IsNil() {
		val = val.Elem()
	}

	if b, ok := asBuilder(data); ok {
		b.build(parentPath)
		return
	}

	if val.Kind() == reflect.Struct {
		for i := range val.NumField() {
			f := val.Field(i)
			if f.CanInterface() {
				reflectBuild(f.Interface(), parentPath)
			}
		}
	}
}

func asBuilder(data any) (builder, bool) {
	b, ok := data.(builder)
	return b, ok
}
