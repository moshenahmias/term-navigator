package fakefs

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/moshenahmias/term-navigator/internal/file"
)

func TestExplorer_BasicOperations(t *testing.T) {
	ctx := context.Background()
	exp := NewExplorer()

	t.Run("Type", func(t *testing.T) {
		if exp.Type() != "fakefs" {
			t.Errorf("expected type 'fakefs', got %q", exp.Type())
		}
	})

	t.Run("DeviceID", func(t *testing.T) {
		if exp.DeviceID(ctx) != "fakefs" {
			t.Errorf("expected deviceID 'fakefs', got %q", exp.DeviceID(ctx))
		}
	})

	t.Run("Cwd", func(t *testing.T) {
		if exp.Cwd(ctx) != "/" {
			t.Errorf("expected cwd '/', got %q", exp.Cwd(ctx))
		}
	})

	t.Run("IsRoot", func(t *testing.T) {
		if !exp.IsRoot(ctx) {
			t.Error("expected IsRoot to be true at root")
		}
	})

	t.Run("Parent at root", func(t *testing.T) {
		parent, ok := exp.Parent(ctx)
		if ok {
			t.Errorf("expected Parent to return false at root, got parent %q", parent)
		}
	})
}

func TestExplorer_DirectoryOperations(t *testing.T) {
	ctx := context.Background()
	exp := NewExplorer()

	t.Run("Mkdir", func(t *testing.T) {
		if err := exp.Mkdir(ctx, "/testdir"); err != nil {
			t.Errorf("Mkdir failed: %v", err)
		}

		if !exp.Exists(ctx, "/testdir") {
			t.Error("directory should exist after Mkdir")
		}
	})

	t.Run("Chdir", func(t *testing.T) {
		if err := exp.Chdir(ctx, "/testdir"); err != nil {
			t.Errorf("Chdir failed: %v", err)
		}

		if exp.Cwd(ctx) != "/testdir" {
			t.Errorf("expected cwd '/testdir', got %q", exp.Cwd(ctx))
		}
	})

	t.Run("Chdir to non-existent", func(t *testing.T) {
		err := exp.Chdir(ctx, "/nonexistent")
		if err == nil {
			t.Error("expected error when chdir to non-existent directory")
		}
	})

	t.Run("Parent", func(t *testing.T) {
		exp.Chdir(ctx, "/testdir")
		parent, ok := exp.Parent(ctx)
		if !ok {
			t.Error("expected Parent to return true")
		}
		if parent != "/" {
			t.Errorf("expected parent '/', got %q", parent)
		}
	})
}

func TestExplorer_FileOperations(t *testing.T) {
	ctx := context.Background()
	exp := NewExplorer()

	exp.Mkdir(ctx, "/dir")
	exp.Write(ctx, "/dir/file.txt", &testReader{data: []byte("hello world")})

	t.Run("Write and Read", func(t *testing.T) {
		rc, err := exp.Read(ctx, "/dir/file.txt")
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}
		defer rc.Close()

		data := make([]byte, 100)
		n, err := rc.Read(data)
		if err != nil {
			t.Fatalf("Read data failed: %v", err)
		}
		data = data[:n]

		if string(data) != "hello world" {
			t.Errorf("expected 'hello world', got %q", string(data))
		}
	})

	t.Run("Read non-existent file", func(t *testing.T) {
		_, err := exp.Read(ctx, "/nonexistent.txt")
		if err == nil {
			t.Error("expected error when reading non-existent file")
		}
	})

	t.Run("Read directory", func(t *testing.T) {
		_, err := exp.Read(ctx, "/dir")
		if err == nil {
			t.Error("expected error when reading directory")
		}
	})
}

func TestExplorer_Delete(t *testing.T) {
	ctx := context.Background()
	exp := NewExplorer()

	exp.Mkdir(ctx, "/dir")
	exp.Write(ctx, "/dir/file.txt", &testReader{data: []byte("test")})

	t.Run("Delete file", func(t *testing.T) {
		if err := exp.Delete(ctx, "/dir/file.txt"); err != nil {
			t.Errorf("Delete failed: %v", err)
		}

		if exp.Exists(ctx, "/dir/file.txt") {
			t.Error("file should not exist after Delete")
		}
	})

	t.Run("Delete directory", func(t *testing.T) {
		if err := exp.Delete(ctx, "/dir"); err != nil {
			t.Errorf("Delete directory failed: %v", err)
		}

		if exp.Exists(ctx, "/dir") {
			t.Error("directory should not exist after Delete")
		}
	})

	t.Run("Delete root", func(t *testing.T) {
		err := exp.Delete(ctx, "/")
		if err == nil {
			t.Error("expected error when deleting root")
		}
	})
}

func TestExplorer_Rename(t *testing.T) {
	ctx := context.Background()
	exp := NewExplorer()

	exp.Mkdir(ctx, "/dir")
	exp.Write(ctx, "/dir/old.txt", &testReader{data: []byte("test")})

	t.Run("Rename file", func(t *testing.T) {
		if err := exp.Rename(ctx, "/dir/old.txt", "/dir/new.txt"); err != nil {
			t.Errorf("Rename failed: %v", err)
		}

		if exp.Exists(ctx, "/dir/old.txt") {
			t.Error("old file should not exist after Rename")
		}
		if !exp.Exists(ctx, "/dir/new.txt") {
			t.Error("new file should exist after Rename")
		}
	})

	t.Run("Rename directory", func(t *testing.T) {
		if err := exp.Rename(ctx, "/dir", "/newdir"); err != nil {
			t.Errorf("Rename directory failed: %v", err)
		}

		if exp.Exists(ctx, "/dir") {
			t.Error("old directory should not exist after Rename")
		}
		if !exp.Exists(ctx, "/newdir") {
			t.Error("new directory should exist after Rename")
		}
	})

	t.Run("Rename non-existent", func(t *testing.T) {
		err := exp.Rename(ctx, "/nonexistent", "/somewhere")
		if err == nil {
			t.Error("expected error when renaming non-existent file")
		}
	})
}

func TestExplorer_List(t *testing.T) {
	ctx := context.Background()
	exp := NewExplorer()

	exp.Mkdir(ctx, "/dir")
	exp.Write(ctx, "/dir/file1.txt", &testReader{data: []byte("a")})
	exp.Write(ctx, "/dir/file2.txt", &testReader{data: []byte("bb")})
	exp.Mkdir(ctx, "/dir/subdir")

	t.Run("List directory", func(t *testing.T) {
		exp.Chdir(ctx, "/dir")
		items, err := exp.List(ctx)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		if len(items) != 3 {
			t.Errorf("expected 3 items, got %d", len(items))
		}

		names := make(map[string]bool)
		for _, item := range items {
			names[item.Name] = true
		}

		expected := map[string]bool{"file1.txt": true, "file2.txt": true, "subdir": true}
		for name := range expected {
			if !names[name] {
				t.Errorf("expected to find %q in listing", name)
			}
		}
	})

	t.Run("List root", func(t *testing.T) {
		exp.Chdir(ctx, "/")
		items, err := exp.List(ctx)
		if err != nil {
			t.Fatalf("List root failed: %v", err)
		}

		if len(items) != 1 {
			t.Errorf("expected 1 item (dir), got %d", len(items))
		}
	})
}

func TestExplorer_Stat(t *testing.T) {
	ctx := context.Background()
	exp := NewExplorer()

	exp.Mkdir(ctx, "/testdir")
	exp.Write(ctx, "/test.txt", &testReader{data: []byte("hello")})

	t.Run("Stat file", func(t *testing.T) {
		info, err := exp.Stat(ctx, "/test.txt")
		if err != nil {
			t.Fatalf("Stat failed: %v", err)
		}

		if info.Name != "test.txt" {
			t.Errorf("expected name 'test.txt', got %q", info.Name)
		}
		if info.IsDir {
			t.Error("expected IsDir to be false for file")
		}
		if info.Size != 5 {
			t.Errorf("expected size 5, got %d", info.Size)
		}
	})

	t.Run("Stat directory", func(t *testing.T) {
		info, err := exp.Stat(ctx, "/testdir")
		if err != nil {
			t.Fatalf("Stat directory failed: %v", err)
		}

		if !info.IsDir {
			t.Error("expected IsDir to be true for directory")
		}
	})

	t.Run("Stat non-existent", func(t *testing.T) {
		_, err := exp.Stat(ctx, "/nonexistent")
		if err == nil {
			t.Error("expected error when stat non-existent")
		}
	})
}

func TestExplorer_Download(t *testing.T) {
	ctx := context.Background()
	exp := NewExplorer()

	exp.Write(ctx, "/file.txt", &testReader{data: []byte("test content")})

	t.Run("Download file", func(t *testing.T) {
		temp, err := exp.Download(ctx, "/file.txt", nil)
		if err != nil {
			t.Fatalf("Download failed: %v", err)
		}
		defer temp.Close()

		data, err := os.ReadFile(temp.Path())
		if err != nil {
			t.Fatalf("Read temp file failed: %v", err)
		}

		if string(data) != "test content" {
			t.Errorf("expected 'test content', got %q", string(data))
		}
	})

	t.Run("Download with progress", func(t *testing.T) {
		var lastProgress int64
		progress := func(name string, n, total int64) {
			lastProgress = n
		}

		temp, err := exp.Download(ctx, "/file.txt", progress)
		if err != nil {
			t.Fatalf("Download with progress failed: %v", err)
		}
		temp.Close()

		if lastProgress == 0 {
			t.Error("expected progress callback to be called")
		}
	})

	t.Run("Download directory", func(t *testing.T) {
		_, err := exp.Download(ctx, "/", nil)
		if err == nil {
			t.Error("expected error when downloading directory")
		}
	})
}

func TestExplorer_Mkdir_Errors(t *testing.T) {
	ctx := context.Background()
	exp := NewExplorer()

	t.Run("Mkdir already exists", func(t *testing.T) {
		exp.Mkdir(ctx, "/existing")
		err := exp.Mkdir(ctx, "/existing")
		if err == nil {
			t.Error("expected error when mkdir already exists")
		}
	})

	t.Run("Mkdir nested", func(t *testing.T) {
		if err := exp.Mkdir(ctx, "/parent"); err != nil {
			t.Fatalf("Mkdir parent failed: %v", err)
		}
		if err := exp.Mkdir(ctx, "/parent/child"); err != nil {
			t.Fatalf("Mkdir nested failed: %v", err)
		}
		if !exp.Exists(ctx, "/parent/child") {
			t.Error("nested directory should exist")
		}
	})
}

func TestExplorer_Copy(t *testing.T) {
	ctx := context.Background()
	exp := NewExplorer()

	exp.Mkdir(ctx, "/source")
	exp.Write(ctx, "/source/file.txt", &testReader{data: []byte("data")})

	t.Run("Copy creates independent cwd", func(t *testing.T) {
		copied := exp.Copy()

		exp.Chdir(ctx, "/source")
		if exp.Cwd(ctx) != "/source" {
			t.Error("original explorer cwd should be /source")
		}

		if copied.Cwd(ctx) != "/" {
			t.Error("copied explorer should still have original cwd")
		}
	})
}

func TestExplorer_Abs(t *testing.T) {
	ctx := context.Background()

	t.Run("Absolute path", func(t *testing.T) {
		exp := NewExplorer()
		if abs := exp.Abs("/file.txt"); abs != "/file.txt" {
			t.Errorf("expected '/file.txt', got %q", abs)
		}
	})

	t.Run("Relative path", func(t *testing.T) {
		exp := NewExplorer()
		exp.Mkdir(ctx, "/dir")
		if err := exp.Chdir(ctx, "/dir"); err != nil {
			t.Fatalf("Chdir failed: %v", err)
		}
		if abs := exp.Abs("file.txt"); abs != "/dir/file.txt" {
			t.Errorf("expected '/dir/file.txt', got %q", abs)
		}
	})
}

func TestExplorer_Metadata(t *testing.T) {
	ctx := context.Background()
	exp := NewExplorer()

	exp.Write(ctx, "/file.txt", &testReader{data: []byte("hello")})
	exp.Mkdir(ctx, "/dir")

	t.Run("File metadata", func(t *testing.T) {
		meta, err := exp.Metadata(ctx, "/file.txt")
		if err != nil {
			t.Fatalf("Metadata failed: %v", err)
		}

		if meta["Type"] != "file" {
			t.Errorf("expected type 'file', got %q", meta["Type"])
		}
		if meta["Size"] != "5 bytes" {
			t.Errorf("expected size '5 bytes', got %q", meta["Size"])
		}
	})

	t.Run("Directory metadata", func(t *testing.T) {
		meta, err := exp.Metadata(ctx, "/dir")
		if err != nil {
			t.Fatalf("Metadata directory failed: %v", err)
		}

		if meta["Type"] != "directory" {
			t.Errorf("expected type 'directory', got %q", meta["Type"])
		}
	})
}

type testReader struct {
	data []byte
	pos  int
}

func (r *testReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

var _ file.Explorer = (*explorer)(nil)
