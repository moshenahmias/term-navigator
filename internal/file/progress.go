package file

import (
	"context"
	"io"
)

type TotalReadFunc func(int64)

type progressReader struct {
	r        io.Reader
	count    int64
	callback TotalReadFunc
	ctx      context.Context
}

func (r *progressReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}

	n, err := r.r.Read(p)
	if n > 0 {
		r.count += int64(n)
		if r.callback != nil {
			r.callback(r.count)
		}
	}
	return n, err
}

func AsProgressReader(ctx context.Context, r io.Reader, callback TotalReadFunc) io.Reader {
	return &progressReader{
		r:        r,
		callback: callback,
		ctx:      ctx,
	}
}
