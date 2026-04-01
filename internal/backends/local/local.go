package local

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/moshenahmias/term-navigator/internal/file"
)

type explorer struct {
	cwd string
}

var _ file.Explorer = (*explorer)(nil)

func NewExplorer(startPath string) file.Explorer {
	abs, err := filepath.Abs(startPath)
	if err != nil {
		abs = startPath
	}
	return &explorer{cwd: abs}
}

func (l *explorer) Copy() file.Explorer {
	cp := *l // shallow copy
	return &cp
}

func (l *explorer) DeviceID(context.Context) string {
	return "local"
}

func (l *explorer) Cwd(context.Context) string {
	return l.cwd
}

func (l *explorer) PrintableCwd(ctx context.Context) string {
	return l.Cwd(ctx)
}

func (l *explorer) IsRoot(ctx context.Context) bool {
	return l.Cwd(ctx) == "/"
}

func (l *explorer) Parent(ctx context.Context) (string, bool) {
	if l.IsRoot(ctx) {
		return "", false
	}
	parent := filepath.Dir(l.cwd)
	return parent, true
}

func (e *explorer) Dir(path string) string {
	return filepath.Dir(path)
}

func (e *explorer) Join(dir, name string) string {
	return filepath.Join(dir, name)
}

func (l *explorer) Chdir(ctx context.Context, path string) error {
	abs := path
	if !filepath.IsAbs(path) {
		abs = l.Join(l.cwd, path)
	}

	info, err := os.Stat(abs)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return os.ErrInvalid
	}

	l.cwd = abs
	return nil
}

func (l *explorer) List(ctx context.Context) ([]file.Info, error) {
	entries, err := os.ReadDir(l.cwd)
	if err != nil {
		return nil, err
	}

	out := make([]file.Info, 0, len(entries))
	for _, e := range entries {
		full := l.Join(l.cwd, e.Name())

		lstat, err := os.Lstat(full)
		if err != nil {
			continue
		}

		isSymlink := lstat.Mode()&os.ModeSymlink != 0
		isDir := e.IsDir() // this follows symlink, but we’ll fix below

		isSymlinkToDir := false
		if isSymlink {
			// follow the symlink
			target, err := os.Stat(full)
			if err == nil && target.IsDir() {
				isSymlinkToDir = true
				isDir = true // treat symlink-to-dir as a directory
			}
		}

		out = append(out, file.Info{
			Name:           e.Name(),
			FullPath:       full,
			IsDir:          isDir,
			Size:           lstat.Size(),
			Modified:       lstat.ModTime(),
			IsSymlink:      isSymlink,
			IsSymlinkToDir: isSymlinkToDir,
		})

	}

	return out, nil
}

func (l *explorer) Stat(ctx context.Context, path string) (file.Info, error) {
	abs := path
	if !filepath.IsAbs(path) {
		abs = l.Join(l.cwd, path)
	}

	fi, err := os.Stat(abs)
	if err != nil {
		return file.Info{}, err
	}

	isSymlink := fi.Mode()&os.ModeSymlink != 0
	isSymlinkToDir := false
	if isSymlink {
		target, err := os.Stat(path) // follows symlink
		if err == nil && target.IsDir() {
			isSymlinkToDir = true
		}
	}

	return file.Info{
		Name:           filepath.Base(abs),
		FullPath:       abs,
		IsDir:          fi.IsDir(),
		Size:           fi.Size(),
		Modified:       fi.ModTime(),
		Extra:          nil,
		IsSymlink:      isSymlink,
		IsSymlinkToDir: isSymlinkToDir,
	}, nil
}

func (l *explorer) Exists(ctx context.Context, path string) bool {
	_, err := l.Stat(ctx, path)
	return err == nil
}

func (l *explorer) Read(_ context.Context, path string) (io.ReadCloser, error) {
	abs := path
	if !filepath.IsAbs(path) {
		abs = filepath.Join(l.cwd, path)
	}
	return os.Open(abs)
}

func (l *explorer) Write(ctx context.Context, path string, r io.Reader) error {
	abs := path
	if !filepath.IsAbs(path) {
		abs = l.Join(l.cwd, path)
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(l.Dir(abs), 0755); err != nil {
		return err
	}

	f, err := os.Create(abs)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, r)
	return err
}

func (l *explorer) Delete(ctx context.Context, path string) error {
	abs := path
	if !filepath.IsAbs(path) {
		abs = l.Join(l.cwd, path)
	}
	return os.RemoveAll(abs)
}

func (l *explorer) Mkdir(ctx context.Context, path string) error {
	abs := path
	if !filepath.IsAbs(path) {
		abs = l.Join(l.cwd, path)
	}
	return os.MkdirAll(abs, 0755)
}

func (l *explorer) Rename(ctx context.Context, oldPath, newPath string) error {
	absOld := oldPath
	if !filepath.IsAbs(oldPath) {
		absOld = l.Join(l.cwd, oldPath)
	}

	absNew := newPath
	if !filepath.IsAbs(newPath) {
		absNew = l.Join(l.cwd, newPath)
	}

	return os.Rename(absOld, absNew)
}

func (l *explorer) Download(ctx context.Context, path string) (file.Temp, error) {
	if path == "" {
		return nil, errors.New("invalid path")
	}

	return file.AsFakeTemp(path), nil
}

func (l *explorer) UploadFrom(ctx context.Context, localPath, destPath string) error {
	if localPath == "" || destPath == "" {
		return errors.New("invalid path")
	}

	// No-op if source and destination are identical
	if localPath == destPath {
		return nil
	}

	info, err := os.Lstat(localPath)
	if err != nil {
		return err
	}

	// -----------------------------
	// CASE 0: Uploading a symlink
	// -----------------------------
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(localPath)
		if err != nil {
			return err
		}

		// Convert relative symlink to absolute
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(localPath), target)
			target = filepath.Clean(target)
		}

		// Ensure destination directory exists
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		return os.Symlink(target, destPath)
	}

	// -----------------------------
	// CASE 1: Uploading a directory
	// -----------------------------
	if info.IsDir() {
		return filepath.Walk(localPath, func(p string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Compute relative path inside the directory
			rel, err := filepath.Rel(localPath, p)
			if err != nil {
				return err
			}

			target := filepath.Join(destPath, rel)

			if fi.IsDir() {
				// Create directory in destination
				return os.MkdirAll(target, 0755)
			}

			// Upload file
			src, err := os.Open(p)
			if err != nil {
				return err
			}
			defer src.Close()

			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}

			return l.Write(ctx, target, src)
		})
	}

	// -----------------------------
	// CASE 2: Uploading a single file
	// -----------------------------
	// Ensure destination directory exists
	if err := os.MkdirAll(l.Dir(destPath), 0755); err != nil {
		return err
	}

	src, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer src.Close()

	return l.Write(ctx, destPath, src)
}

func (l *explorer) Metadata(ctx context.Context, path string) (map[string]string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}

	meta := make(map[string]string)

	// Basic info
	meta["Name"] = info.Name()
	meta["Size"] = fmt.Sprintf("%d bytes", info.Size())
	meta["Modified"] = info.ModTime().Format("2006-01-02 15:04:05")

	// File mode (permissions)
	mode := info.Mode()
	meta["Mode"] = mode.String() // e.g. "-rw-r--r--"
	meta["Mode (octal)"] = fmt.Sprintf("%04o", mode.Perm())

	// Type
	switch {
	case mode.IsRegular():
		meta["Type"] = "file"
	case mode.IsDir():
		meta["Type"] = "directory"
	case mode&os.ModeSymlink != 0:
		meta["Type"] = "symlink"
		if target, err := os.Readlink(path); err == nil {
			meta["Target"] = target
		}
	default:
		meta["Type"] = mode.Type().String()
	}

	// Owner info (Unix only)
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		meta["UID"] = fmt.Sprintf("%d", stat.Uid)
		meta["GID"] = fmt.Sprintf("%d", stat.Gid)
	}

	return meta, nil
}
