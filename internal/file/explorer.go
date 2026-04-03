package file

import (
	"context"
	"io"
	"time"
)

type Info struct {
	Name           string
	FullPath       string
	IsDir          bool
	Size           int64
	Modified       time.Time
	Extra          map[string]any
	IsSymlink      bool
	IsSymlinkToDir bool
}

type Temp interface {
	Path() string
	Close() error
}

type ProgressFunc func(name string, n, total int64)

type Explorer interface {
	Copy() Explorer
	DeviceID(ctx context.Context) string
	Cwd(ctx context.Context) string
	PrintableCwd(ctx context.Context) string
	IsRoot(ctx context.Context) bool
	Parent(ctx context.Context) (string, bool)
	Dir(path string) string
	Join(dir, name string) string
	Chdir(ctx context.Context, path string) error
	List(ctx context.Context) ([]Info, error)
	Stat(ctx context.Context, path string) (Info, error)
	Exists(ctx context.Context, path string) bool
	Read(ctx context.Context, path string) (io.ReadCloser, error)
	Write(ctx context.Context, path string, r io.Reader) error
	Delete(ctx context.Context, path string) error
	Mkdir(ctx context.Context, path string) error
	Rename(ctx context.Context, oldPath, newPath string) error
	Download(ctx context.Context, path string) (Temp, error)
	UploadFrom(ctx context.Context, localPath, destPath string, progress ProgressFunc) error
	Metadata(ctx context.Context, path string) (map[string]string, error)
}
