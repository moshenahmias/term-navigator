package file

import "os"

type realTemp struct {
	path string
}

var _ Temp = (*realTemp)(nil)

func (t *realTemp) Path() string { return t.path }

func (t *realTemp) Close() error {
	if t.path == "" {
		return nil
	}
	err := os.Remove(t.path)
	t.path = ""
	return err
}

func AsRealTemp(path string) Temp {
	return &realTemp{
		path: path,
	}
}

type fakeTemp struct {
	path string
}

var _ Temp = fakeTemp{}

func (h fakeTemp) Path() string { return h.path }
func (h fakeTemp) Close() error { return nil } // no-op

func AsFakeTemp(path string) Temp {
	return &fakeTemp{
		path: path,
	}
}
