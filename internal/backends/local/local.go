package local

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/moshenahmias/term-navigator/internal/explorer"
)

type localTempFileHandle struct {
	path string
}

var _ explorer.TempFileHandle = localTempFileHandle{}

func (h localTempFileHandle) Path() string { return h.path }
func (h localTempFileHandle) Close() error { return nil } // no-op

type LocalExplorer struct {
	cwd string
}

var _ explorer.FileExplorer = (*LocalExplorer)(nil)

func NewLocalExplorer(startPath string) *LocalExplorer {
	abs, err := filepath.Abs(startPath)
	if err != nil {
		abs = startPath
	}
	return &LocalExplorer{cwd: abs}
}

func (l *LocalExplorer) Cwd() string {
	return l.cwd
}

func (l *LocalExplorer) Chdir(path string) error {
	abs := path
	if !filepath.IsAbs(path) {
		abs = filepath.Join(l.cwd, path)
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

func (l *LocalExplorer) List() ([]explorer.FileInfo, error) {
	entries, err := os.ReadDir(l.cwd)
	if err != nil {
		return nil, err
	}

	out := make([]explorer.FileInfo, 0, len(entries))
	for _, e := range entries {
		full := filepath.Join(l.cwd, e.Name())

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

		out = append(out, explorer.FileInfo{
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

func (l *LocalExplorer) Stat(path string) (explorer.FileInfo, error) {
	abs := path
	if !filepath.IsAbs(path) {
		abs = filepath.Join(l.cwd, path)
	}

	fi, err := os.Stat(abs)
	if err != nil {
		return explorer.FileInfo{}, err
	}

	isSymlink := fi.Mode()&os.ModeSymlink != 0
	isSymlinkToDir := false
	if isSymlink {
		target, err := os.Stat(path) // follows symlink
		if err == nil && target.IsDir() {
			isSymlinkToDir = true
		}
	}

	return explorer.FileInfo{
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

func (l *LocalExplorer) Exists(path string) bool {
	_, err := l.Stat(path)
	return err == nil
}

func (l *LocalExplorer) Read(path string) (io.ReadCloser, error) {
	abs := path
	if !filepath.IsAbs(path) {
		abs = filepath.Join(l.cwd, path)
	}
	return os.Open(abs)
}

func (l *LocalExplorer) Write(path string, r io.Reader) error {
	abs := path
	if !filepath.IsAbs(path) {
		abs = filepath.Join(l.cwd, path)
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
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

func (l *LocalExplorer) Delete(path string) error {
	abs := path
	if !filepath.IsAbs(path) {
		abs = filepath.Join(l.cwd, path)
	}
	return os.RemoveAll(abs)
}

func (l *LocalExplorer) Mkdir(path string) error {
	abs := path
	if !filepath.IsAbs(path) {
		abs = filepath.Join(l.cwd, path)
	}
	return os.MkdirAll(abs, 0755)
}

func (l *LocalExplorer) Rename(oldPath, newPath string) error {
	absOld := oldPath
	if !filepath.IsAbs(oldPath) {
		absOld = filepath.Join(l.cwd, oldPath)
	}

	absNew := newPath
	if !filepath.IsAbs(newPath) {
		absNew = filepath.Join(l.cwd, newPath)
	}

	return os.Rename(absOld, absNew)
}

func (l *LocalExplorer) Download(path string) (explorer.TempFileHandle, error) {
	if path == "" {
		return nil, errors.New("invalid path")
	}

	return localTempFileHandle{path: path}, nil
}

func (l *LocalExplorer) UploadFrom(localPath, destPath string) error {
	if localPath == "" || destPath == "" {
		return errors.New("invalid path")
	}

	// No-op if source and destination are identical
	if localPath == destPath {
		return nil
	}

	// Ensure destination directory exists
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	// Open source file
	src, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer src.Close()

	// Delegate actual writing to backend's Write()
	return l.Write(destPath, src)
}
