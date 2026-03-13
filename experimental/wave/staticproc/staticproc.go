package staticproc

import (
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/kit/set"
	"github.com/vormadev/vorma/kit/strict"
)

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
		string(sp.SrcDir),
		func(path string, d fs.DirEntry, walk_err error) error {
			if walk_err != nil {
				return walk_err
			}
			if d.IsDir() {
				return nil
			}

			rel, err := filepath.Rel(string(sp.SrcDir), path)
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

			if _, exists := entries[logical]; exists {
				return fmt.Errorf(
					"collision: multiple source files map to %q",
					logical,
				)
			}

			info, err := d.Info()
			if err != nil {
				return err
			}

			// Reuse cached hash when the file was NOT in the watcher's
			// changed set and mtime+size are unchanged.
			if prev != nil &&
				(changed == nil || !changed.Has(strict.CWDRelPath(path))) {
				if old, ok := prev.entries[logical]; ok &&
					old.mod_time.Equal(info.ModTime()) &&
					old.size == info.Size() {
					out_name := output_name(output_name_args{
						logical:         logical,
						hash_with_key:   old.hash_with_key,
						is_passthrough:  is_passthrough,
						out_file_prefix: sp.OutFilePrefix,
					})
					entries[logical] = file_entry{
						src_path:       strict.CWDRelPath(path),
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
				src_path:       strict.CWDRelPath(path),
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
	result := make(map[string]string, len(fm.entries))
	for logical, entry := range fm.entries {
		rel, _ := filepath.Rel(string(fm.out_dir), string(entry.out_path))
		result[logical] = filepath.ToSlash(rel)
	}
	return result
}

// Plan compares the filemap against the output directory and returns what
// needs to be copied and what needs to be deleted. All paths on the result
// are CWD-relative.
func (fm *Filemap) Plan() (PlanResult, error) {
	var result PlanResult

	expected := set.New[strict.CWDRelPath]()
	for logical, entry := range fm.entries {
		expected.Add(entry.out_path)

		needed, err := file_needs_copy(entry, string(entry.out_path), logical)
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
		string(fm.out_dir),
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
		if err := os.Remove(string(path)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	for _, cp := range p.should_copy {
		if err := os.MkdirAll(filepath.Dir(string(cp.out_path)), 0o755); err != nil {
			return err
		}
		if cp.data != nil {
			if err := os.WriteFile(string(cp.out_path), cp.data, 0o644); err != nil {
				return err
			}
		} else {
			if err := copy_file(string(cp.src_path), string(cp.out_path)); err != nil {
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

func copy_file(src, out string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out_file, err := os.Create(out)
	if err != nil {
		return err
	}
	defer out_file.Close()
	if _, err := io.Copy(out_file, in); err != nil {
		return err
	}
	return out_file.Close()
}

func (fm *Filemap) WriteFilemapJSON(out_path strict.CWDRelPath) error {
	_map := fm.Map()
	kv_slice := make([][2]string, 0, len(_map))
	for k, v := range _map {
		kv_slice = append(kv_slice, [2]string{k, v})
	}
	slices.SortFunc(kv_slice, func(a, b [2]string) int {
		return strings.Compare(a[0], b[0])
	})
	sb := strings.Builder{}
	sb.WriteString("{\n")
	for i, tup := range kv_slice {
		sb.WriteString("\t")
		sb.WriteString(`"` + tup[0] + `"`)
		sb.WriteString(": ")
		sb.WriteString(`"` + tup[1] + `"`)
		if i < len(kv_slice)-1 {
			sb.WriteString(",\n")
		} else {
			sb.WriteString("\n")
		}
	}
	sb.WriteString("}\n")
	if err := fsutil.EnsureDir(filepath.Dir(string(out_path))); err != nil {
		return fmt.Errorf("failed to ensure dir for filemap JSON: %w", err)
	}
	if err := os.WriteFile(string(out_path), []byte(sb.String()), 0644); err != nil {
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
