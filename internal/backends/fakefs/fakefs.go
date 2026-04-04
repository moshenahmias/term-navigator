package fakefs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/moshenahmias/term-navigator/internal/file"
)

//
// ────────────────────────────────────────────────────────────────
//   Node structure
// ────────────────────────────────────────────────────────────────
//

type node struct {
	name     string
	isDir    bool
	data     []byte
	children map[string]*node
	modTime  time.Time
	extra    map[string]any
}

func newDir(name string) *node {
	return &node{
		name:     name,
		isDir:    true,
		children: make(map[string]*node),
		modTime:  time.Now(),
		extra:    make(map[string]any),
	}
}

func newFile(name string, data []byte) *node {
	return &node{
		name:     name,
		isDir:    false,
		data:     data,
		children: nil,
		modTime:  time.Now(),
		extra:    make(map[string]any),
	}
}

//
// ────────────────────────────────────────────────────────────────
//   Temp implementation
// ────────────────────────────────────────────────────────────────
//

type temp struct {
	path string
}

func (t temp) Path() string { return t.path }
func (t temp) Close() error { return nil }

//
// ────────────────────────────────────────────────────────────────
//   Explorer
// ────────────────────────────────────────────────────────────────
//

type Explorer struct {
	mu   sync.RWMutex
	cwd  string
	root *node
}

var _ file.Explorer = (*Explorer)(nil)

func NewExplorer() *Explorer {
	return &Explorer{
		cwd:  "/",
		root: newDir("/"),
	}
}

func (e *Explorer) Copy() file.Explorer {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return &Explorer{
		// fresh zero-value mutex
		cwd:  e.cwd,
		root: e.root, // share the same tree
	}
}

func (e *Explorer) Type() string { return "fakefs" }

func (e *Explorer) DeviceID(context.Context) string { return e.Type() }

func (e *Explorer) Cwd(context.Context) string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.cwd
}

func (e *Explorer) PrintableCwd(ctx context.Context) string {
	return e.Cwd(ctx)
}

func (e *Explorer) IsRoot(ctx context.Context) bool {
	return e.Cwd(ctx) == "/"
}

func (e *Explorer) Parent(ctx context.Context) (string, bool) {
	if e.IsRoot(ctx) {
		return "", false
	}
	return path.Dir(e.Cwd(ctx)), true
}

func (e *Explorer) Dir(p string) string     { return path.Dir(p) }
func (e *Explorer) Join(a, b string) string { return path.Join(a, b) }

func clean(p string) string {
	if p == "" {
		return "/"
	}
	c := path.Clean(p)
	if !strings.HasPrefix(c, "/") {
		c = "/" + c
	}
	return c
}

func (e *Explorer) absPath(p string) string {
	if path.IsAbs(p) {
		return clean(p)
	}
	return clean(path.Join(e.cwd, p))
}

//
// ────────────────────────────────────────────────────────────────
//   Unlocked lookup helpers (NO LOCKS INSIDE)
// ────────────────────────────────────────────────────────────────
//

func (e *Explorer) lookupUnlocked(p string) (*node, error) {
	if p == "/" {
		return e.root, nil
	}

	parts := strings.Split(strings.TrimPrefix(p, "/"), "/")
	cur := e.root

	for _, part := range parts {
		if !cur.isDir {
			return nil, errors.New("not a directory")
		}
		child, ok := cur.children[part]
		if !ok {
			return nil, os.ErrNotExist
		}
		cur = child
	}
	return cur, nil
}

func (e *Explorer) ensureParentUnlocked(p string) (*node, string, error) {
	dir := path.Dir(p)
	base := path.Base(p)

	parent, err := e.lookupUnlocked(dir)
	if err != nil {
		return nil, "", err
	}
	if !parent.isDir {
		return nil, "", errors.New("parent is not a directory")
	}
	return parent, base, nil
}

//
// ────────────────────────────────────────────────────────────────
//   Chdir
// ────────────────────────────────────────────────────────────────
//

func (e *Explorer) Chdir(ctx context.Context, p string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	abs := e.absPath(p)
	n, err := e.lookupUnlocked(abs)
	if err != nil {
		return err
	}
	if !n.isDir {
		return errors.New("not a directory")
	}

	e.cwd = abs
	return nil
}

//
// ────────────────────────────────────────────────────────────────
//   List
// ────────────────────────────────────────────────────────────────
//

func (e *Explorer) List(ctx context.Context) ([]file.Info, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	n, err := e.lookupUnlocked(e.cwd)
	if err != nil {
		return nil, err
	}
	if !n.isDir {
		return nil, errors.New("not a directory")
	}

	out := make([]file.Info, 0, len(n.children))
	for _, c := range n.children {
		out = append(out, file.Info{
			Name:     c.name,
			FullPath: path.Join(e.cwd, c.name),
			IsDir:    c.isDir,
			Size:     int64(len(c.data)),
			Modified: c.modTime,
			Extra:    c.extra,
		})
	}
	return out, nil
}

//
// ────────────────────────────────────────────────────────────────
//   Stat
// ────────────────────────────────────────────────────────────────
//

func (e *Explorer) Stat(ctx context.Context, p string) (file.Info, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	abs := e.absPath(p)
	n, err := e.lookupUnlocked(abs)
	if err != nil {
		return file.Info{}, err
	}

	return file.Info{
		Name:     path.Base(abs),
		FullPath: abs,
		IsDir:    n.isDir,
		Size:     int64(len(n.data)),
		Modified: n.modTime,
		Extra:    n.extra,
	}, nil
}

func (e *Explorer) Exists(ctx context.Context, p string) bool {
	_, err := e.Stat(ctx, p)
	return err == nil
}

//
// ────────────────────────────────────────────────────────────────
//   Read
// ────────────────────────────────────────────────────────────────
//

func (e *Explorer) Read(ctx context.Context, p string) (io.ReadCloser, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	abs := e.absPath(p)
	n, err := e.lookupUnlocked(abs)
	if err != nil {
		return nil, err
	}
	if n.isDir {
		return nil, errors.New("cannot read directory")
	}

	return io.NopCloser(bytes.NewReader(n.data)), nil
}

//
// ────────────────────────────────────────────────────────────────
//   Write
// ────────────────────────────────────────────────────────────────
//

func (e *Explorer) Write(ctx context.Context, p string, r io.Reader) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	abs := e.absPath(p)
	parent, base, err := e.ensureParentUnlocked(abs)
	if err != nil {
		return err
	}

	buf, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	parent.children[base] = newFile(base, buf)
	return nil
}

//
// ────────────────────────────────────────────────────────────────
//   Delete
// ────────────────────────────────────────────────────────────────
//

func (e *Explorer) Delete(ctx context.Context, p string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	abs := e.absPath(p)
	if abs == "/" {
		return errors.New("cannot delete root")
	}

	parent, base, err := e.ensureParentUnlocked(abs)
	if err != nil {
		return err
	}

	delete(parent.children, base)
	return nil
}

//
// ────────────────────────────────────────────────────────────────
//   Mkdir
// ────────────────────────────────────────────────────────────────
//

func (e *Explorer) Mkdir(ctx context.Context, p string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	abs := e.absPath(p)
	parent, base, err := e.ensureParentUnlocked(abs)
	if err != nil {
		return err
	}

	if _, exists := parent.children[base]; exists {
		return errors.New("already exists")
	}

	parent.children[base] = newDir(base)
	return nil
}

//
// ────────────────────────────────────────────────────────────────
//   Rename
// ────────────────────────────────────────────────────────────────
//

func (e *Explorer) Rename(ctx context.Context, oldPath, newPath string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	oldAbs := e.absPath(oldPath)
	newAbs := e.absPath(newPath)

	oldParent, oldBase, err := e.ensureParentUnlocked(oldAbs)
	if err != nil {
		return err
	}

	n, ok := oldParent.children[oldBase]
	if !ok {
		return os.ErrNotExist
	}

	newParent, newBase, err := e.ensureParentUnlocked(newAbs)
	if err != nil {
		return err
	}

	delete(oldParent.children, oldBase)
	n.name = newBase
	newParent.children[newBase] = n

	return nil
}

//
// ────────────────────────────────────────────────────────────────
//   Download
// ────────────────────────────────────────────────────────────────
//

func (e *Explorer) Download(ctx context.Context, p string, progress file.ProgressFunc) (file.Temp, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	abs := e.absPath(p)
	n, err := e.lookupUnlocked(abs)
	if err != nil {
		return nil, err
	}
	if n.isDir {
		return nil, errors.New("cannot download a directory")
	}

	// Create a real OS temp file
	f, err := os.CreateTemp("", "term-nav-fakefs-*")
	if err != nil {
		return nil, err
	}

	var r io.Reader = bytes.NewReader(n.data)

	if progress != nil {
		size := int64(len(n.data))
		r = file.AsProgressReader(ctx, r, func(n int64) {
			progress(p, n, size)
		})
	}

	// Write the in-memory file contents
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, err
	}

	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return nil, err
	}

	return file.AsRealTemp(file.TempOpts{Path: f.Name()}), nil
}

//
// ────────────────────────────────────────────────────────────────
//   UploadFrom (real FS → fake FS)
// ────────────────────────────────────────────────────────────────
//

func (e *Explorer) UploadFrom(ctx context.Context, localPath, destPath string, progress file.ProgressFunc) error {
	info, err := os.Lstat(localPath)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return e.uploadDir(ctx, localPath, destPath, progress)
	}

	return e.uploadFile(ctx, localPath, destPath, func(n int64) {
		progress(localPath, n, info.Size())
	})
}

func (e *Explorer) uploadFile(ctx context.Context, localPath, destPath string, progress file.TotalReadFunc) error {
	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer f.Close()

	var pr io.Reader = f

	if progress != nil {
		pr = file.AsProgressReader(ctx, f, progress)
	}

	return e.Write(ctx, destPath, pr)
}

func (e *Explorer) uploadDir(ctx context.Context, localPath, destPath string, progress file.ProgressFunc) error {
	return filepathWalk(localPath, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		rel := strings.TrimPrefix(p, localPath)
		rel = strings.TrimPrefix(rel, "/")
		target := path.Join(destPath, rel)

		if fi.IsDir() {
			return e.Mkdir(ctx, target)
		}

		return e.uploadFile(ctx, p, target, func(n int64) {
			progress(p, n, fi.Size())
		})
	})
}

func filepathWalk(root string, fn func(string, os.FileInfo, error) error) error {
	info, err := os.Lstat(root)
	if err != nil {
		return fn(root, nil, err)
	}

	if err := fn(root, info, nil); err != nil {
		return err
	}

	if !info.IsDir() {
		return nil
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return fn(root, nil, err)
	}

	for _, e := range entries {
		p := path.Join(root, e.Name())
		if err := filepathWalk(p, fn); err != nil {
			return err
		}
	}

	return nil
}

//
// ────────────────────────────────────────────────────────────────
//   Metadata
// ────────────────────────────────────────────────────────────────
//

func (e *Explorer) Metadata(ctx context.Context, p string) (map[string]string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	abs := e.absPath(p)
	n, err := e.lookupUnlocked(abs)
	if err != nil {
		return nil, err
	}

	meta := map[string]string{
		"Name":     n.name,
		"Modified": n.modTime.Format("2006-01-02 15:04:05"),
	}

	if n.isDir {
		meta["Type"] = "directory"
	} else {
		meta["Type"] = "file"
		meta["Size"] = fmt.Sprintf("%d bytes", len(n.data))
	}

	return meta, nil
}
