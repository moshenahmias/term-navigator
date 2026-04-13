package file

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
)

func TestLazyDevice_Get(t *testing.T) {
	ctx := context.Background()
	callCount := atomic.Int32{}

	constructor := func(ctx context.Context) (map[string]Explorer, error) {
		callCount.Add(1)
		return map[string]Explorer{
			"dev1": &mockExplorer{name: "dev1"},
			"dev2": &mockExplorer{name: "dev2"},
		}, nil
	}

	lazy := NewLazyDevice(constructor)

	t.Run("lazy initialization", func(t *testing.T) {
		if callCount.Load() != 0 {
			t.Error("constructor should not be called before Get")
		}
	})

	t.Run("first Get calls constructor", func(t *testing.T) {
		devices, err := lazy.Get(ctx)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		if len(devices) != 2 {
			t.Errorf("expected 2 devices, got %d", len(devices))
		}

		if callCount.Load() != 1 {
			t.Errorf("constructor should be called once, got %d", callCount.Load())
		}
	})

	t.Run("subsequent Get returns cached", func(t *testing.T) {
		devices1, _ := lazy.Get(ctx)
		devices2, _ := lazy.Get(ctx)

		if devices1 == nil || devices2 == nil {
			t.Error("should return non-nil maps")
		}

		if len(devices1) != len(devices2) {
			t.Error("should return maps with same content")
		}

		if callCount.Load() != 1 {
			t.Errorf("constructor should still be called once, got %d", callCount.Load())
		}
	})
}

func TestLazyDevice_Error(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("connection failed")

	constructor := func(ctx context.Context) (map[string]Explorer, error) {
		return nil, expectedErr
	}

	lazy := NewLazyDevice(constructor)

	t.Run("Get returns error", func(t *testing.T) {
		_, err := lazy.Get(ctx)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("InitError returns error", func(t *testing.T) {
		if lazy.InitError() != expectedErr {
			t.Error("InitError should return the error")
		}
	})
}

func TestLazyDevice_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	callCount := atomic.Int32{}

	constructor := func(ctx context.Context) (map[string]Explorer, error) {
		callCount.Add(1)
		return map[string]Explorer{"dev": &mockExplorer{name: "dev"}}, nil
	}

	lazy := NewLazyDevice(constructor)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lazy.Get(ctx)
		}()
	}
	wg.Wait()

	if callCount.Load() != 1 {
		t.Errorf("constructor should be called once, got %d", callCount.Load())
	}
}

type mockExplorer struct {
	name string
}

func (m *mockExplorer) Type() string                                 { return m.name }
func (m *mockExplorer) Copy() Explorer                               { return m }
func (m *mockExplorer) DeviceID(ctx context.Context) string          { return "" }
func (m *mockExplorer) Cwd(ctx context.Context) string               { return "" }
func (m *mockExplorer) PrintableCwd(ctx context.Context) string      { return "" }
func (m *mockExplorer) IsRoot(ctx context.Context) bool              { return false }
func (m *mockExplorer) Parent(ctx context.Context) (string, bool)    { return "", false }
func (m *mockExplorer) Dir(path string) string                       { return "" }
func (m *mockExplorer) Join(dir, name string) string                 { return "" }
func (m *mockExplorer) Chdir(ctx context.Context, path string) error { return nil }
func (m *mockExplorer) List(ctx context.Context) ([]Info, error)     { return nil, nil }
func (m *mockExplorer) Stat(ctx context.Context, path string) (Info, error) {
	return Info{}, nil
}
func (m *mockExplorer) Exists(ctx context.Context, path string) bool { return false }
func (m *mockExplorer) Read(ctx context.Context, path string) (io.ReadCloser, error) {
	return nil, nil
}
func (m *mockExplorer) Write(ctx context.Context, path string, r io.Reader) error { return nil }
func (m *mockExplorer) Delete(ctx context.Context, path string) error             { return nil }
func (m *mockExplorer) Mkdir(ctx context.Context, path string) error              { return nil }
func (m *mockExplorer) Rename(ctx context.Context, oldPath, newPath string) error {
	return nil
}
func (m *mockExplorer) Download(ctx context.Context, path string, progress ProgressFunc) (Temp, error) {
	return nil, nil
}
func (m *mockExplorer) UploadFrom(ctx context.Context, localPath, destPath string, progress ProgressFunc) error {
	return nil
}
func (m *mockExplorer) Metadata(ctx context.Context, path string) (map[string]string, error) {
	return nil, nil
}
func (m *mockExplorer) Abs(path string) string { return "" }

var _ Explorer = (*mockExplorer)(nil)
