package staticproc

import (
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/kit/jsonutil"
	"github.com/vormadev/vorma/kit/set"
	"github.com/vormadev/vorma/kit/strict"
)

var static_files_ignore = map[string]struct{}{
	".DS_Store": {},
}

type file_entry struct {
	src_path       strict.CWDRelPath
	bytes          []byte
	out_path       strict.CWDRelPath
	hash_with_key  string
	is_passthrough bool
	mod_time       time.Time
	size           int64
}

type MemFiles map[string][]byte

type StaticProcessor struct {
	SrcDir              strict.CWDRelPath
	OutDir              strict.CWDRelPath
	PassthroughDirnames []string
	OutFilePrefix       string
}

func (sp *StaticProcessor) PhantomFilemap() *Filemap {
	return &Filemap{
		entries:         make(map[string]file_entry),
		out_dir:         sp.OutDir,
		out_file_prefix: sp.OutFilePrefix,
	}
}

type Filemap struct {
	mu              sync.RWMutex
	entries         map[string]file_entry
	out_dir         strict.CWDRelPath
	out_file_prefix string
}

func (fm *Filemap) OutDir() strict.CWDRelPath {
	return fm.out_dir
}

type should_copy struct {
	src_path strict.CWDRelPath
	data     []byte
	out_path strict.CWDRelPath
}

type PlanResult struct {
	should_copy   []should_copy
	should_delete []strict.CWDRelPath
}

// PhysicalFilemap walks the source directory, hashes every file, and returns a Filemap
// keyed by logical relative path. Directories whose names appear in
// PassthroughDirnames are flagged as passthrough and their prefix is stripped
// from the logical path. All paths on entries are CWD-relative.
//
// If prev is non-nil, PhysicalFilemap reuses cached hashes for files that were not
// reported as changed (via the changed set) and whose mtime+size are
// unchanged. On the initial build, pass nil for both prev and changed.
func (sp *StaticProcessor) PhysicalFilemap(
	prev *Filemap,
	changed *set.Set[strict.CWDRelPath],
) (*Filemap, error) {
	passthrough_set := set.New(sp.PassthroughDirnames)
	entries := make(map[string]file_entry)

	err := filepath.WalkDir(
		sp.SrcDir.Str(),
		func(path string, d fs.DirEntry, walk_err error) error {
			if walk_err != nil {
				return walk_err
			}
			if d.IsDir() {
				return nil
			}
			if _, ignored := static_files_ignore[d.Name()]; ignored {
				return nil
			}

			rel, err := filepath.Rel(sp.SrcDir.Str(), path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)

			is_passthrough := false
			logical := rel
			if before, after, ok := strings.Cut(rel, "/"); ok {
				if passthrough_set.Has(before) {
					is_passthrough = true
					logical = after
				}
			}
			if logical == "" {
				return nil
			}

			if existing, exists := entries[logical]; exists {
				return fmt.Errorf(
					"collision: multiple source files map to %q (sources: %q and %q)",
					logical,
					existing.src_path,
					path,
				)
			}

			info, err := d.Info()
			if err != nil {
				return err
			}
			src_path := strict.MustNormalize(path)

			// Reuse cached hash when the file was NOT in the watcher's
			// changed set and mtime+size are unchanged.
			if prev != nil &&
				(changed == nil || !changed.Has(src_path)) {
				if old, ok := prev.entries[logical]; ok &&
					old.src_path == src_path &&
					old.mod_time.Equal(info.ModTime()) &&
					old.size == info.Size() {
					out_name := output_name(output_name_args{
						logical:         logical,
						hash_with_key:   old.hash_with_key,
						is_passthrough:  is_passthrough,
						out_file_prefix: sp.OutFilePrefix,
					})
					entries[logical] = file_entry{
						src_path:       src_path,
						out_path:       sp.OutDir.Join(out_name),
						hash_with_key:  old.hash_with_key,
						is_passthrough: is_passthrough,
						mod_time:       info.ModTime(),
						size:           info.Size(),
					}
					return nil
				}
			}

			hash_with_key, err := hash_file_with_key(path, logical)
			if err != nil {
				return err
			}

			out_name := output_name(output_name_args{
				logical:         logical,
				hash_with_key:   hash_with_key,
				is_passthrough:  is_passthrough,
				out_file_prefix: sp.OutFilePrefix,
			})
			entries[logical] = file_entry{
				src_path:       src_path,
				out_path:       sp.OutDir.Join(out_name),
				hash_with_key:  hash_with_key,
				is_passthrough: is_passthrough,
				mod_time:       info.ModTime(),
				size:           info.Size(),
			}
			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return &Filemap{
		entries:         entries,
		out_dir:         sp.OutDir,
		out_file_prefix: sp.OutFilePrefix,
	}, nil
}

// Set merges in-memory files into the filemap, overwriting any existing
// entries with the same logical path.
func (fm *Filemap) Set(mem_files MemFiles) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	for key, bytes := range mem_files {
		logical := filepath.ToSlash(key)
		if logical == "" {
			continue
		}

		hash_with_key := hash_bytes_with_key(bytes, logical)
		out_name := output_name(output_name_args{
			logical:         logical,
			hash_with_key:   hash_with_key,
			is_passthrough:  false,
			out_file_prefix: fm.out_file_prefix,
		})

		fm.entries[logical] = file_entry{
			bytes:         bytes,
			out_path:      fm.out_dir.Join(out_name),
			hash_with_key: hash_with_key,
		}
	}
}

// Map returns a map of logical path to output-directory-relative path.
func (fm *Filemap) Map() map[string]string {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	result := make(map[string]string, len(fm.entries))
	for logical, entry := range fm.entries {
		rel, _ := filepath.Rel(fm.out_dir.Str(), entry.out_path.Str())
		result[logical] = filepath.ToSlash(rel)
	}
	return result
}

// Plan compares the filemap against the output directory and returns what
// needs to be copied and what needs to be deleted. All paths on the result
// are CWD-relative.
func (fm *Filemap) Plan() (PlanResult, error) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	var result PlanResult

	expected := set.New[strict.CWDRelPath]()
	for logical, entry := range fm.entries {
		expected.Add(entry.out_path)

		needed, err := file_needs_copy(entry, entry.out_path.Str(), logical)
		if err != nil {
			return result, err
		}
		if needed {
			result.should_copy = append(result.should_copy, should_copy{
				src_path: entry.src_path,
				data:     entry.bytes,
				out_path: entry.out_path,
			})
		}
	}

	walk_err := filepath.WalkDir(
		fm.out_dir.Str(),
		func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			if !expected.Has(strict.CWDRelPath(path)) {
				result.should_delete = append(
					result.should_delete,
					strict.CWDRelPath(path),
				)
			}
			return nil
		},
	)
	if walk_err != nil && !os.IsNotExist(walk_err) {
		return result, walk_err
	}

	return result, nil
}

// Apply executes the PlanResult.
func (p PlanResult) Apply() error {
	for _, path := range p.should_delete {
		if err := os.Remove(path.Str()); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	for _, sc := range p.should_copy {
		if err := fsutil.EnsureDir(filepath.Dir(sc.out_path.Str())); err != nil {
			return err
		}
		if sc.data != nil {
			if err := os.WriteFile(sc.out_path.Str(), sc.data, 0o644); err != nil {
				return err
			}
		} else {
			if err := fsutil.CopyFile(sc.src_path.Str(), sc.out_path.Str()); err != nil {
				return err
			}
		}
	}
	return nil
}

type output_name_args struct {
	logical         string
	hash_with_key   string
	is_passthrough  bool
	out_file_prefix string
}

// output_name derives the output filename (not a full path).
// Hashed:      {out_file_prefix}{flattened_path}_{hash12}{ext}
// Passthrough: original relative path unchanged.
func output_name(args output_name_args) string {
	if args.is_passthrough {
		return args.logical
	}
	ext := filepath.Ext(args.logical)
	stem := strings.TrimSuffix(args.logical, ext)
	flat := strings.ReplaceAll(stem, "/", "_")
	short_hash := args.hash_with_key[:12]
	return args.out_file_prefix + flat + "_" + short_hash + ext
}

func file_needs_copy(
	entry file_entry,
	out_path string,
	logical string,
) (bool, error) {
	_, err := os.Stat(out_path)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if !entry.is_passthrough {
		return false, nil
	}
	out_hash, err := hash_file_with_key(out_path, logical)
	if err != nil {
		return false, err
	}
	return out_hash != entry.hash_with_key, nil
}

func hash_file_with_key(path string, key string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	h.Write([]byte(key))
	h.Write([]byte{0})
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return strings.ToLower(
		base32.StdEncoding.WithPadding(base32.NoPadding).
			EncodeToString(h.Sum(nil)),
	), nil
}

func hash_bytes_with_key(data []byte, key string) string {
	h := sha256.New()
	h.Write([]byte(key))
	h.Write([]byte{0})
	h.Write(data)
	return strings.ToLower(
		base32.StdEncoding.WithPadding(base32.NoPadding).
			EncodeToString(h.Sum(nil)),
	)
}

func (fm *Filemap) WriteFilemapJSON(out_path strict.CWDRelPath) error {
	_map := fm.Map()
	data, err := jsonutil.SerializePretty(_map)
	if err != nil {
		return fmt.Errorf("failed to encode filemap JSON: %w", err)
	}
	if err := fsutil.EnsureDir(out_path.Dir().Str()); err != nil {
		return fmt.Errorf("failed to ensure dir for filemap JSON: %w", err)
	}
	if err := os.WriteFile(out_path.Str(), append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("failed to write filemap JSON: %w", err)
	}
	return nil
}

func (fm *Filemap) ApplyDiffAndWriteFilemapJSON(
	json_out_path strict.CWDRelPath,
) error {
	diff_result, err := fm.Plan()
	if err != nil {
		return fmt.Errorf("static diff plan failed: %w", err)
	}
	if err := diff_result.Apply(); err != nil {
		return fmt.Errorf("static diff apply failed: %w", err)
	}
	return fm.WriteFilemapJSON(json_out_path)
}
