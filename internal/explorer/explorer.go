package explorer

import (
	"io"
	"time"
)

type FileInfo struct {
	Name           string
	FullPath       string
	IsDir          bool
	Size           int64
	Modified       time.Time
	Extra          map[string]any
	IsSymlink      bool
	IsSymlinkToDir bool
}

type TempFileHandle interface {
	Path() string
	Close() error
}

type FileExplorer interface {
	Cwd() string
	Chdir(path string) error

	List() ([]FileInfo, error)
	Stat(path string) (FileInfo, error)
	Exists(path string) bool

	Read(path string) (io.ReadCloser, error)
	Write(path string, r io.Reader) error

	Delete(path string) error
	Mkdir(path string) error
	Rename(oldPath, newPath string) error

	Download(path string) (TempFileHandle, error)
	UploadFrom(localPath, destPath string) error
}
