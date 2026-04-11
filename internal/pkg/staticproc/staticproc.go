package staticproc

import (
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vormadev/vorma/kit/bytesutil"
	"github.com/vormadev/vorma/kit/cryptoutil"
	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/kit/lru"
	"github.com/vormadev/vorma/kit/matcher"
	"github.com/vormadev/vorma/kit/set"
)

var ignored = set.New([]string{".DS_Store"})

type hash_cache_key struct {
	src_path string
	mod_time time.Time
	size     int64
}

var hash_cache = lru.NewCache[hash_cache_key, string](10_000)

type Files map[string]*File // SrcPathRel -> File

type File struct {
	SrcDir           string
	SrcPathRel       string // Relative to SrcDir
	SrcNamePrehashed bool
	Bytes            []byte
	OutNamePrefix    string
	ModTime          time.Time
	OutName          string
	Size             int64
}

func (files Files) Inject(file *File) {
	files[file.SrcPathRel] = file
	files[file.SrcPathRel].Hash()
}

func (files Files) Map() map[string]string {
	m := make(map[string]string)
	for rel, f := range files {
		m[rel] = f.OutName
	}
	return m
}

func CollectPhysical(
	src_dir string,
	prehashed_dirs []string,
	out_name_prefix string,
) (Files, error) {
	passthroughs := set.New(prehashed_dirs)
	files := make(Files)

	err := filepath.WalkDir(src_dir, func(
		path string, d fs.DirEntry, walk_err error,
	) error {
		if walk_err != nil {
			return walk_err
		}
		if d.IsDir() {
			return nil
		}
		if ignored.Has(d.Name()) {
			return nil
		}

		rel, err := filepath.Rel(src_dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		files[rel] = &File{
			SrcDir:        src_dir,
			SrcPathRel:    rel,
			OutNamePrefix: out_name_prefix,
		}
		if passthroughs.Has("") ||
			passthroughs.Has(".") ||
			passthroughs.Has(matcher.ParseSegments(rel)[0]) {
			files[rel].SrcNamePrehashed = true
		}

		if err := files[rel].Hash(); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}

func Reconcile(out_dir string, files Files) error {
	target_paths := set.New[string]()

	var to_copy []*File
	for _, f := range files {
		out_path := filepath.Join(out_dir, f.OutName)
		target_paths.Add(out_path)

		if _, err := os.Stat(out_path); os.IsNotExist(err) {
			to_copy = append(to_copy, f)
		} else if err != nil {
			return err
		}
	}

	// Delete files in out_dir that are not in the current filemap.
	walk_err := filepath.WalkDir(out_dir, func(
		path string, d fs.DirEntry, err error,
	) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !target_paths.Has(path) {
			return os.Remove(path)
		}
		return nil
	})
	if walk_err != nil && !os.IsNotExist(walk_err) {
		return walk_err
	}

	// Copy files that are missing.
	for _, f := range to_copy {
		out_path := filepath.Join(out_dir, f.OutName)
		if err := fsutil.EnsureDir(filepath.Dir(out_path)); err != nil {
			return err
		}
		is_physical := f.Bytes == nil
		if is_physical {
			src_path := filepath.Join(f.SrcDir, f.SrcPathRel)
			if err := fsutil.CopyFile(src_path, out_path); err != nil {
				return err
			}
		} else {
			if err := os.WriteFile(out_path, f.Bytes, 0o644); err != nil {
				return err
			}
		}
	}

	return nil
}

func (f *File) Hash() error {
	if f.SrcPathRel == "" {
		return errors.New("empty SrcPath")
	}

	is_physical := f.Bytes == nil

	needs_hash := f.OutName == "" || !is_physical

	src_path := ""
	if is_physical {
		src_path = filepath.Join(f.SrcDir, f.SrcPathRel)
		info, err := os.Stat(src_path)
		if err != nil {
			return err
		}
		if f.ModTime != info.ModTime() {
			needs_hash = true
			f.ModTime = info.ModTime()
		}
		if f.Size != info.Size() {
			needs_hash = true
			f.Size = info.Size()
		}
	}

	if !needs_hash {
		return nil
	}

	h := sha256.New()
	h.Write([]byte(f.SrcPathRel))
	h.Write([]byte{0})

	if is_physical {
		ch, err := hash_physical(hash_cache_key{
			src_path: src_path,
			mod_time: f.ModTime,
			size:     f.Size,
		})
		if err != nil {
			return err
		}
		h.Write([]byte(ch))
	} else {
		h.Write([]byte(hash_virtual(f.Bytes)))
	}

	f.OutName = f.OutNamePrefix
	if f.SrcNamePrehashed {
		f.OutName += f.SrcPathRel
	} else {
		ext := filepath.Ext(f.SrcPathRel)
		stem := filepath.ToSlash(strings.TrimSuffix(f.SrcPathRel, ext))
		flat := strings.ReplaceAll(stem, "/", "_")
		enc := base32.StdEncoding.WithPadding(base32.NoPadding)
		trunc_hash := strings.ToLower(enc.EncodeToString(h.Sum(nil)))[:12]
		f.OutName += flat + "_" + trunc_hash + ext
	}

	return nil
}

func hash_physical(ck hash_cache_key) (string, error) {
	if hash, ok := hash_cache.Get(ck); ok {
		return hash, nil
	}

	f, err := os.Open(ck.src_path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	ch := bytesutil.ToBase64(h.Sum(nil))
	hash_cache.Set(ck, ch)

	return ch, nil
}

func hash_virtual(data []byte) string {
	return bytesutil.ToBase64(cryptoutil.Sha256Hash(data))
}
