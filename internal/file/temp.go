package file

import "os"

type TempOpts struct {
	Path string
	Dest func(string) string
}

type baseTemp struct {
	path string
	dest func(string) string
}

func (t *baseTemp) Path() string {
	return t.path
}

func (t *baseTemp) Dest(name string) string {
	if t.dest == nil {
		return name
	}
	return t.dest(name)
}

type realTemp struct {
	baseTemp
}

func (t *realTemp) Close() error {
	if t.path == "" {
		return nil
	}
	err := os.Remove(t.path)
	t.path = ""
	return err
}

func AsRealTemp(opts TempOpts) Temp {
	return &realTemp{
		baseTemp: baseTemp{
			path: opts.Path,
			dest: opts.Dest,
		},
	}
}

type fakeTemp struct {
	baseTemp
}

func (t *fakeTemp) Close() error {
	return nil
}

func AsFakeTemp(opts TempOpts) Temp {
	return &fakeTemp{
		baseTemp: baseTemp{
			path: opts.Path,
			dest: opts.Dest,
		},
	}
}
