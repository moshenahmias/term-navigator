package local

import (
	"io"
	"os"
	"path/filepath"

	"github.com/moshenahmias/term-navigator/internal/explorer"
)

type LocalExplorer struct {
	cwd string
}

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
		fi, err := e.Info()
		if err != nil {
			continue
		}

		out = append(out, explorer.FileInfo{
			Name:     e.Name(),
			FullPath: filepath.Join(l.cwd, e.Name()),
			IsDir:    e.IsDir(),
			Size:     fi.Size(),
			Modified: fi.ModTime(),
			Extra:    nil,
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

	return explorer.FileInfo{
		Name:     filepath.Base(abs),
		FullPath: abs,
		IsDir:    fi.IsDir(),
		Size:     fi.Size(),
		Modified: fi.ModTime(),
		Extra:    nil,
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
