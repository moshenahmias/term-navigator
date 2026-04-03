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
	if r == nil || callback == nil {
		return r
	}

	return &progressReader{
		r:        r,
		callback: callback,
		ctx:      ctx,
	}
}

type progressReaderSeeker struct {
	*progressReader
	seeker io.Seeker
}

func (p *progressReaderSeeker) Seek(offset int64, whence int) (int64, error) {
	// If S3 rewinds to the beginning, reset progress
	if whence == io.SeekStart && offset == 0 {
		p.count = 0
	}
	return p.seeker.Seek(offset, whence)
}

func AsProgressReadSeeker(ctx context.Context, r io.ReadSeeker, callback TotalReadFunc) io.ReadSeeker {
	if r == nil || callback == nil {
		return r
	}

	return &progressReaderSeeker{
		progressReader: &progressReader{
			r:        r,
			callback: callback,
			ctx:      ctx,
		},
		seeker: r,
	}
}
