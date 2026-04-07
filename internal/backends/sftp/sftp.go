package sftp

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/moshenahmias/term-navigator/internal/file"
	"github.com/pkg/sftp"
)

type explorer struct {
	client *sftp.Client
	cwd    string
	host   string
}

var _ file.Explorer = (*explorer)(nil)

const deviceType = "sftp"

func NewExplorer(client *sftp.Client, host, startPath string) file.Explorer {
	abs := startPath
	if !strings.HasPrefix(abs, "/") {
		abs = "/" + abs
	}
	return &explorer{
		client: client,
		cwd:    filepath.Clean(abs),
		host:   host,
	}
}

func (e *explorer) Copy() file.Explorer {
	cp := *e
	return &cp
}

func (e *explorer) Type() string { return deviceType }

func (e *explorer) DeviceID(ctx context.Context) string {
	return fmt.Sprintf("sftp:%s", e.host)
}

func (e *explorer) Cwd(context.Context) string { return e.cwd }

func (e *explorer) PrintableCwd(ctx context.Context) string {
	return fmt.Sprintf("sftp://%s%s", e.host, e.cwd)
}

func (e *explorer) IsRoot(ctx context.Context) bool {
	return e.cwd == "/"
}

func (e *explorer) Parent(ctx context.Context) (string, bool) {
	if e.cwd == "/" {
		return "", false
	}
	p := filepath.Dir(e.cwd)
	if p == "." {
		p = "/"
	}
	return p, true
}

func (e *explorer) Dir(p string) string {
	return filepath.Dir(p)
}

func (e *explorer) Join(dir, name string) string {
	return filepath.Join(dir, name)
}

func (e *explorer) Chdir(ctx context.Context, p string) error {
	var abs string
	if filepath.IsAbs(p) {
		abs = p
	} else {
		abs = filepath.Join(e.cwd, p)
	}

	st, err := e.client.Stat(abs)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return os.ErrInvalid
	}

	e.cwd = abs
	return nil
}

func (e *explorer) List(ctx context.Context) ([]file.Info, error) {
	entries, err := e.client.ReadDir(e.cwd)
	if err != nil {
		return nil, err
	}

	out := make([]file.Info, 0, len(entries))
	for _, fi := range entries {
		full := filepath.Join(e.cwd, fi.Name())

		mode := fi.Mode()
		isSymlink := mode&os.ModeSymlink != 0
		isDir := fi.IsDir()

		// Resolve symlink target
		isSymlinkToDir := false
		if isSymlink {
			target, err := e.client.Stat(full)
			if err == nil && target.IsDir() {
				isSymlinkToDir = true
				isDir = true
			}
		}

		out = append(out, file.Info{
			Name:           fi.Name(),
			FullPath:       full,
			IsDir:          isDir,
			Size:           fi.Size(),
			Modified:       fi.ModTime(),
			IsSymlink:      isSymlink,
			IsSymlinkToDir: isSymlinkToDir,
		})
	}

	return out, nil
}

func (e *explorer) Stat(ctx context.Context, p string) (file.Info, error) {
	var abs string
	if filepath.IsAbs(p) {
		abs = p
	} else {
		abs = filepath.Join(e.cwd, p)
	}

	fi, err := e.client.Stat(abs)
	if err != nil {
		return file.Info{}, err
	}

	mode := fi.Mode()
	isSymlink := mode&os.ModeSymlink != 0
	isSymlinkToDir := false
	isDir := fi.IsDir()

	if isSymlink {
		target, err := e.client.Stat(abs)
		if err == nil && target.IsDir() {
			isSymlinkToDir = true
			isDir = true
		}
	}

	return file.Info{
		Name:           filepath.Base(abs),
		FullPath:       abs,
		IsDir:          isDir,
		Size:           fi.Size(),
		Modified:       fi.ModTime(),
		IsSymlink:      isSymlink,
		IsSymlinkToDir: isSymlinkToDir,
	}, nil
}

func (e *explorer) Exists(ctx context.Context, p string) bool {
	_, err := e.Stat(ctx, p)
	return err == nil
}

func (e *explorer) Read(ctx context.Context, p string) (io.ReadCloser, error) {
	abs := p
	if !filepath.IsAbs(p) {
		abs = filepath.Join(e.cwd, p)
	}
	return e.client.Open(abs)
}

func (e *explorer) Write(ctx context.Context, p string, r io.Reader) error {
	abs := p
	if !filepath.IsAbs(p) {
		abs = filepath.Join(e.cwd, p)
	}

	// Ensure parent exists
	if err := e.client.MkdirAll(filepath.Dir(abs)); err != nil {
		return err
	}

	f, err := e.client.Create(abs)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, r)
	return err
}

func (e *explorer) Delete(ctx context.Context, p string) error {
	abs := p
	if !filepath.IsAbs(p) {
		abs = filepath.Join(e.cwd, p)
	}

	fi, err := e.client.Stat(abs)
	if err != nil {
		return err
	}

	if fi.IsDir() {
		return e.client.RemoveDirectory(abs)
	}
	return e.client.Remove(abs)
}

func (e *explorer) Mkdir(ctx context.Context, p string) error {
	abs := p
	if !filepath.IsAbs(p) {
		abs = filepath.Join(e.cwd, p)
	}
	return e.client.MkdirAll(abs)
}

func (e *explorer) Rename(ctx context.Context, oldPath, newPath string) error {
	var src, dst string
	if filepath.IsAbs(oldPath) {
		src = oldPath
	} else {
		src = filepath.Join(e.cwd, oldPath)
	}
	if filepath.IsAbs(newPath) {
		dst = newPath
	} else {
		dst = filepath.Join(e.cwd, newPath)
	}
	return e.client.Rename(src, dst)
}

func (e *explorer) Download(ctx context.Context, p string, progress file.ProgressFunc) (file.Temp, error) {
	// SFTP: just read the file into a temp
	abs := p
	if !filepath.IsAbs(p) {
		abs = filepath.Join(e.cwd, p)
	}

	rc, err := e.client.Open(abs)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	tmp, err := os.CreateTemp("", "sftp-download-*")
	if err != nil {
		return nil, err
	}

	if progress != nil {
		pr := file.AsProgressReader(ctx, rc, func(n int64) {
			progress(abs, n, 0)
		})
		_, err = io.Copy(tmp, pr)
	} else {
		_, err = io.Copy(tmp, rc)
	}

	if err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, err
	}

	tmp.Close()
	return file.AsRealTemp(file.TempOpts{Path: tmp.Name()}), nil
}

func (e *explorer) UploadFrom(ctx context.Context, localPath, destPath string, progress file.ProgressFunc) error {
	info, err := os.Stat(localPath)
	if err != nil {
		return err
	}

	var dst string
	if filepath.IsAbs(destPath) {
		dst = destPath
	} else {
		dst = filepath.Join(e.cwd, destPath)
	}

	if info.IsDir() {
		return filepath.Walk(localPath, func(p string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			rel, err := filepath.Rel(localPath, p)
			if err != nil {
				return err
			}

			target := filepath.Join(dst, rel)

			if fi.IsDir() {
				return e.client.MkdirAll(target)
			}

			src, err := os.Open(p)
			if err != nil {
				return err
			}
			defer src.Close()

			if err := e.client.MkdirAll(filepath.Dir(target)); err != nil {
				return err
			}

			f, err := e.client.Create(target)
			if err != nil {
				return err
			}
			defer f.Close()

			if progress != nil {
				pr := file.AsProgressReader(ctx, src, func(n int64) {
					progress(fi.Name(), n, fi.Size())
				})
				_, err = io.Copy(f, pr)
			} else {
				_, err = io.Copy(f, src)
			}

			return err
		})
	}

	// Single file
	src, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer src.Close()

	if err := e.client.MkdirAll(filepath.Dir(dst)); err != nil {
		return err
	}

	f, err := e.client.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()

	if progress != nil {
		pr := file.AsProgressReader(ctx, src, func(n int64) {
			progress(info.Name(), n, info.Size())
		})
		_, err = io.Copy(f, pr)
	} else {
		_, err = io.Copy(f, src)
	}

	return err
}

func (e *explorer) Metadata(ctx context.Context, p string) (map[string]string, error) {
	abs := p
	if !filepath.IsAbs(p) {
		abs = filepath.Join(e.cwd, p)
	}

	fi, err := e.client.Stat(abs)
	if err != nil {
		return nil, err
	}

	m := map[string]string{
		"Name":     filepath.Base(abs),
		"FullPath": abs,
		"Size":     fmt.Sprintf("%d", fi.Size()),
		"Modified": fi.ModTime().Format(time.RFC3339),
		"Mode":     fi.Mode().String(),
	}

	return m, nil
}
