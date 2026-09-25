package backup

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

const MaxBytes int64 = 1 << 30

type reader struct {
	ctx context.Context
	r   io.Reader
}

func (r reader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

// Pack creates a private temporary ZIP. Its root represents the supplied home.
// File symlinks are dereferenced; their contents retain the link's ZIP path.
// Directory symlinks are rejected, and no symlinks are stored in the archive.
func Pack(ctx context.Context, home string, roots []string) (name string, err error) {
	entries := map[string]os.FileInfo{}
	var total int64
	for _, root := range roots {
		if e := safeParents(filepath.Dir(root)); e != nil {
			return "", e
		}
		rel, e := filepath.Rel(home, root)
		if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("caminho fora de ~: %s", root)
		}
		e = filepath.Walk(root, func(p string, info os.FileInfo, e error) error {
			if e != nil {
				return e
			}
			if e = ctx.Err(); e != nil {
				return e
			}
			if info.Mode()&os.ModeSymlink != 0 {
				info, e = os.Stat(p)
				if e != nil {
					return fmt.Errorf("resolver link %s: %w", p, e)
				}
				if !info.Mode().IsRegular() {
					return fmt.Errorf("link não aponta para arquivo regular: %s", p)
				}
			}
			if !info.IsDir() && !info.Mode().IsRegular() {
				return fmt.Errorf("arquivo especial não suportado: %s", p)
			}
			if _, ok := entries[p]; !ok {
				total += info.Size()
				entries[p] = info
			}
			if total > MaxBytes {
				return fmt.Errorf("backup excede limite de 1 GiB")
			}
			return nil
		})
		if e != nil {
			return "", e
		}
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("nenhum caminho selecionado")
	}
	file, err := os.CreateTemp("", "holdotfiles-*.zip")
	if err != nil {
		return "", err
	}
	name = file.Name()
	defer func() {
		file.Close()
		if err != nil {
			os.Remove(name)
		}
	}()
	zw := zip.NewWriter(file)
	names := make([]string, 0, len(entries))
	for p := range entries {
		names = append(names, p)
	}
	sort.Strings(names)
	for _, p := range names {
		info := entries[p]
		rel, _ := filepath.Rel(home, p)
		if rel == "." {
			continue
		}
		header, e := zip.FileInfoHeader(info)
		if e != nil {
			return name, e
		}
		header.Name = filepath.ToSlash(rel)
		header.Method = zip.Deflate
		if info.IsDir() {
			header.Name += "/"
			header.Method = zip.Store
		}
		out, e := zw.CreateHeader(header)
		if e != nil {
			return name, e
		}
		if info.IsDir() {
			continue
		}
		in, e := os.Open(p)
		if e != nil {
			return name, e
		}
		_, e = io.Copy(out, reader{ctx, in})
		in.Close()
		if e != nil {
			return name, e
		}
	}
	if err = zw.Close(); err != nil {
		return name, err
	}
	err = file.Close()
	return name, err
}

type Entry struct {
	Name    string
	Replace bool
}
type Plan struct{ Entries []Entry }

func inspect(zr *zip.ReadCloser, dest string) (Plan, error) {
	var plan Plan
	seen := map[string]bool{}
	var total uint64
	for _, f := range zr.File {
		name := strings.TrimSuffix(f.Name, "/")
		if name == "" || name == "." || !fsValid(name) || path.Clean(name) != name || strings.Contains(name, "\\") || strings.Contains(name, ":") {
			return plan, fmt.Errorf("caminho inválido no ZIP: %q", f.Name)
		}
		if seen[name] {
			return plan, fmt.Errorf("entrada duplicada: %s", name)
		}
		seen[name] = true
		if !f.Mode().IsRegular() && !f.Mode().IsDir() {
			return plan, fmt.Errorf("tipo não suportado: %s", name)
		}
		if f.UncompressedSize64 > uint64(MaxBytes)-total {
			return plan, fmt.Errorf("ZIP excede limite de 1 GiB")
		}
		total += f.UncompressedSize64
		target := filepath.Join(dest, filepath.FromSlash(name))
		if err := safeParents(target); err != nil {
			return plan, err
		}
		info, err := os.Lstat(target)
		exists := err == nil
		if err != nil && !os.IsNotExist(err) {
			return plan, err
		}
		if exists && info.IsDir() != f.FileInfo().IsDir() {
			return plan, fmt.Errorf("conflito arquivo/diretório: %s", target)
		}
		plan.Entries = append(plan.Entries, Entry{name, exists && !f.FileInfo().IsDir()})
	}
	return plan, nil
}
func fsValid(name string) bool {
	if strings.HasPrefix(name, "/") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." || part == "." || part == "" {
			return false
		}
	}
	return true
}
func safeParents(target string) error {
	for p := target; ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("destino contém link simbólico: %s", p)
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
	}
	return nil
}

func Preview(filename, dest string) (Plan, error) {
	zr, err := zip.OpenReader(filename)
	if err != nil {
		return Plan{}, err
	}
	defer zr.Close()
	return inspect(zr, dest)
}

// Install validates and stages every entry before replacing any destination file.
// Existing files are moved into a unique recovery directory under dest.
func Install(ctx context.Context, filename, dest string) (recovery string, err error) {
	dest, err = filepath.Abs(dest)
	if err != nil {
		return "", err
	}
	zr, err := zip.OpenReader(filename)
	if err != nil {
		return "", err
	}
	defer zr.Close()
	if _, err = inspect(zr, dest); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp("", "holdotfiles-stage-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	for _, f := range zr.File {
		if err = ctx.Err(); err != nil {
			return "", err
		}
		target := filepath.Join(stage, filepath.FromSlash(f.Name))
		if f.FileInfo().IsDir() {
			err = os.MkdirAll(target, 0700)
			if err != nil {
				return "", err
			}
			continue
		}
		if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return "", err
		}
		in, e := f.Open()
		if e != nil {
			return "", e
		}
		out, e := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			in.Close()
			return "", e
		}
		_, e = io.Copy(out, reader{ctx, io.LimitReader(in, MaxBytes+1)})
		in.Close()
		closeErr := out.Close()
		if e != nil {
			return "", e
		}
		if closeErr != nil {
			return "", closeErr
		}
		if err = os.Chmod(target, f.Mode().Perm()); err != nil {
			return "", err
		}
	}
	if err = os.MkdirAll(dest, 0700); err != nil {
		return "", err
	}
	recovery, err = os.MkdirTemp(dest, ".holdotfiles-recovery-")
	if err != nil {
		return "", err
	}
	for _, f := range zr.File {
		if err = ctx.Err(); err != nil {
			return recovery, err
		}
		target := filepath.Join(dest, filepath.FromSlash(f.Name))
		if err = safeParents(target); err != nil {
			return recovery, err
		}
		if f.FileInfo().IsDir() {
			err = os.MkdirAll(target, 0700)
			if err != nil {
				return recovery, err
			}
			continue
		}
		if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return recovery, err
		}
		if _, e := os.Lstat(target); e == nil {
			old := filepath.Join(recovery, filepath.FromSlash(f.Name))
			if err = os.MkdirAll(filepath.Dir(old), 0700); err != nil {
				return recovery, err
			}
			if err = os.Rename(target, old); err != nil {
				return recovery, err
			}
		}
		// Copy to the destination filesystem, then rename atomically.
		in, e := os.Open(filepath.Join(stage, filepath.FromSlash(f.Name)))
		if e != nil {
			return recovery, e
		}
		out, e := os.CreateTemp(filepath.Dir(target), ".holdotfiles-install-")
		if e != nil {
			in.Close()
			return recovery, e
		}
		temp := out.Name()
		_, e = io.Copy(out, in)
		in.Close()
		ce := out.Close()
		if e == nil {
			e = ce
		}
		if e == nil {
			e = os.Chmod(temp, f.Mode().Perm())
		}
		if e == nil {
			e = os.Rename(temp, target)
		}
		if e != nil {
			os.Remove(temp)
			return recovery, e
		}
	}
	return recovery, nil
}
