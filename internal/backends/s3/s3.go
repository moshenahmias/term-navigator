package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/moshenahmias/term-navigator/internal/file"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type explorer struct {
	client   *s3.Client
	bucket   string
	region   string
	endpoint string
	cwd      string // prefix, always ends with "/" or empty
	id       string
}

var _ file.Explorer = (*explorer)(nil)

func NewExplorer(client *s3.Client, endpoint, region, bucket, startPrefix string) file.Explorer {
	p := strings.TrimPrefix(startPrefix, "/")
	if p != "" && !strings.HasSuffix(p, "/") {
		p += "/"
	}

	var id string

	if endpoint != "" {
		id = fmt.Sprintf("s3:%s/%s/%s", endpoint, region, bucket)
	} else {
		id = fmt.Sprintf("s3:%s/%s", region, bucket)
	}

	return &explorer{
		client:   client,
		bucket:   bucket,
		endpoint: endpoint,
		region:   region,
		cwd:      p,
		id:       id,
	}
}

func (e *explorer) Copy() file.Explorer {
	cp := *e // shallow copy
	return &cp
}

func (e *explorer) DeviceID(ctx context.Context) string {
	return e.id
}

func (e *explorer) Cwd(context.Context) string {
	return e.cwd
}

func (e *explorer) PrintableCwd(ctx context.Context) string {
	var s string
	if e.region != "" {
		s = fmt.Sprintf("(%s) %s/%s/%s", e.region, e.endpoint, e.bucket, e.Cwd(ctx))
	} else {
		s = fmt.Sprintf("%s/%s/%s", e.endpoint, e.bucket, e.Cwd(ctx))
	}

	s = strings.ReplaceAll(s, "//", "/")
	return s
}

func (e *explorer) IsRoot(ctx context.Context) bool {
	return e.Cwd(ctx) == ""
}

func (e *explorer) Parent(ctx context.Context) (string, bool) {
	cwd := e.cwd

	// Root has no parent
	if cwd == "" {
		return "", false
	}

	// Remove trailing slash
	trimmed := strings.TrimSuffix(cwd, "/")

	// Find last slash
	idx := strings.LastIndex(trimmed, "/")
	if idx < 0 {
		// Example: "x/" → parent is root ""
		return "", true
	}

	// Extract parent prefix
	parent := trimmed[:idx+1] // already ends with "/"

	// Normalize: collapse accidental double slashes
	for strings.Contains(parent, "//") {
		parent = strings.ReplaceAll(parent, "//", "/")
	}

	return parent, true
}

func (e *explorer) Dir(key string) string {
	trimmed := strings.TrimSuffix(key, "/")

	idx := strings.LastIndex(trimmed, "/")
	if idx < 0 {
		return "" // root
	}

	return trimmed[:idx+1]
}

func (e *explorer) Join(dir, name string) string {
	// Ensure dir ends with "/"
	if dir != "" && !strings.HasSuffix(dir, "/") {
		dir += "/"
	}

	// Preserve trailing slash for directories
	if strings.HasSuffix(name, "/") {
		return dir + name
	}

	return dir + name
}

func (e *explorer) Chdir(ctx context.Context, p string) error {
	var prefix string

	// If p is "" or ends with "/", treat it as an absolute prefix
	if p == "" || strings.HasSuffix(p, "/") {
		prefix = p
	} else {
		// Otherwise treat it as a relative path
		prefix = e.abs(p)
	}

	// Normalize: ensure directories end with "/"
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	// Validate prefix exists
	out, err := e.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(e.bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(1),
	})
	if err != nil {
		return err
	}
	if len(out.Contents) == 0 && len(out.CommonPrefixes) == 0 {
		return errors.New("directory does not exist")
	}

	e.cwd = prefix
	return nil
}

func (e *explorer) List(ctx context.Context) ([]file.Info, error) {
	out, err := e.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:    aws.String(e.bucket),
		Prefix:    aws.String(e.cwd),
		Delimiter: aws.String("/"),
	})
	if err != nil {
		return nil, err
	}

	var items []file.Info

	// "Directories" via CommonPrefixes
	for _, cp := range out.CommonPrefixes {
		name := strings.TrimSuffix(strings.TrimPrefix(aws.ToString(cp.Prefix), e.cwd), "/")
		if name == "" {
			continue
		}
		items = append(items, file.Info{
			Name:     name,
			FullPath: aws.ToString(cp.Prefix),
			IsDir:    true,
			Size:     0,
			Modified: time.Time{},
		})
	}

	// Files
	for _, obj := range out.Contents {
		key := aws.ToString(obj.Key)
		if key == e.cwd {
			continue // skip the "directory" placeholder
		}
		name := strings.TrimPrefix(key, e.cwd)
		if name == "" || strings.Contains(name, "/") {
			continue // deeper level, already represented by CommonPrefixes
		}
		items = append(items, file.Info{
			Name:     name,
			FullPath: key,
			IsDir:    false,
			Size:     *obj.Size,
			Modified: aws.ToTime(obj.LastModified),
		})
	}

	return items, nil
}

func (e *explorer) Stat(ctx context.Context, p string) (file.Info, error) {
	key := e.abs(p)

	// Try as file
	head, err := e.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(e.bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		name := strings.TrimSuffix(key, "/")
		idx := strings.LastIndex(name, "/")
		if idx >= 0 {
			name = name[idx+1:]
		}

		return file.Info{
			Name:     name,
			FullPath: key,
			IsDir:    false,
			Size:     *head.ContentLength,
			Modified: aws.ToTime(head.LastModified),
		}, nil
	}

	// Try as "directory"
	if !strings.HasSuffix(key, "/") {
		key += "/"
	}

	out, err := e.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(e.bucket),
		Prefix:  aws.String(key),
		MaxKeys: aws.Int32(1),
	})
	if err != nil {
		return file.Info{}, err
	}
	if len(out.Contents) == 0 && len(out.CommonPrefixes) == 0 {
		return file.Info{}, errors.New("not found")
	}

	// Extract directory name (prefix-based)
	name := strings.TrimSuffix(key, "/")
	idx := strings.LastIndex(name, "/")
	if idx >= 0 {
		name = name[idx+1:]
	}

	return file.Info{
		Name:     name,
		FullPath: key,
		IsDir:    true,
		Size:     0,
		Modified: time.Time{},
	}, nil
}

func (e *explorer) Exists(ctx context.Context, p string) bool {
	_, err := e.Stat(ctx, p)
	return err == nil
}

func (e *explorer) Read(ctx context.Context, p string) (io.ReadCloser, error) {
	key := e.abs(p)
	out, err := e.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(e.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}

func (e *explorer) Write(ctx context.Context, p string, r io.Reader) error {
	key := e.abs(p)
	_, err := e.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(e.bucket),
		Key:    aws.String(key),
		Body:   r,
	})
	return err
}

func (e *explorer) Delete(ctx context.Context, p string) error {
	key := e.abs(p)

	// Directory: delete all under prefix
	if strings.HasSuffix(key, "/") {
		return e.deletePrefix(ctx, key)
	}

	_, err := e.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(e.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (e *explorer) Mkdir(ctx context.Context, p string) error {
	key := e.abs(p)
	if !strings.HasSuffix(key, "/") {
		key += "/"
	}
	_, err := e.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(e.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(nil),
	})
	return err
}

func (e *explorer) Rename(ctx context.Context, oldPath, newPath string) error {
	src := e.abs(oldPath)
	dst := e.abs(newPath)

	// If it's a "directory", we need to copy all keys under the prefix
	if strings.HasSuffix(src, "/") {
		return e.renamePrefix(ctx, src, dst)
	}

	// Single object rename
	copySource := url.PathEscape(e.bucket + "/" + src)

	// Single object
	_, err := e.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(e.bucket),
		CopySource: aws.String(copySource),
		Key:        aws.String(dst),
	})
	if err != nil {
		return err
	}

	_, err = e.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(e.bucket),
		Key:    aws.String(src),
	})
	return err
}

func (e *explorer) Download(ctx context.Context, p string) (file.Temp, error) {
	key := e.abs(p)

	out, err := e.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(e.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()

	f, err := os.CreateTemp("", "term-nav-s3-*")
	if err != nil {
		return nil, err
	}

	if _, err := io.Copy(f, out.Body); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, err
	}

	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return nil, err
	}

	return file.AsRealTemp(f.Name()), nil
}

func (e *explorer) UploadFrom(ctx context.Context, localPath, destPath string, progress file.ProgressFunc) error {
	if localPath == "" || destPath == "" {
		return errors.New("invalid path")
	}

	info, err := os.Stat(localPath)
	if err != nil {
		return err
	}

	// Directory upload: walk and mirror structure into S3
	if info.IsDir() {
		return filepath.Walk(localPath, func(p string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if ctx.Err() != nil {
				return ctx.Err()
			}

			rel, err := filepath.Rel(localPath, p)
			if err != nil {
				return err
			}

			targetKey := e.abs(path.Join(destPath, filepath.ToSlash(rel)))

			if fi.IsDir() {
				if !strings.HasSuffix(targetKey, "/") {
					targetKey += "/"
				}
				_, err := e.client.PutObject(ctx, &s3.PutObjectInput{
					Bucket: aws.String(e.bucket),
					Key:    aws.String(targetKey),
					Body:   bytes.NewReader(nil),
				})
				return err
			}

			src, err := os.Open(p)
			if err != nil {
				return err
			}
			defer src.Close()

			_, err = e.client.PutObject(ctx, &s3.PutObjectInput{
				Bucket: aws.String(e.bucket),
				Key:    aws.String(targetKey),
				Body: file.AsProgressReadSeeker(ctx, src, func(n int64) {
					progress(p, n, fi.Size())
				}),
			})
			return err
		})
	}

	// Single file
	src, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer src.Close()

	key := e.abs(destPath)
	_, err = e.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(e.bucket),
		Key:    aws.String(key),
		Body: file.AsProgressReadSeeker(ctx, src, func(n int64) {
			progress(localPath, n, info.Size())
		}),
	})
	return err
}

func (e *explorer) abs(key string) string {
	// Already absolute
	if strings.HasPrefix(key, e.cwd) {
		return key
	}

	// Ensure cwd ends with "/"
	cwd := e.cwd
	if cwd != "" && !strings.HasSuffix(cwd, "/") {
		cwd += "/"
	}

	return cwd + key
}

func (e *explorer) deletePrefix(ctx context.Context, prefix string) error {
	var contToken *string

	for {
		out, err := e.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(e.bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: contToken,
		})
		if err != nil {
			return err
		}

		if len(out.Contents) == 0 {
			return nil
		}

		objs := make([]s3types.ObjectIdentifier, 0, len(out.Contents))
		for _, o := range out.Contents {
			objs = append(objs, s3types.ObjectIdentifier{Key: o.Key})
		}

		_, err = e.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(e.bucket),
			Delete: &s3types.Delete{Objects: objs},
		})
		if err != nil {
			return err
		}

		if !*out.IsTruncated {
			return nil
		}
		contToken = out.NextContinuationToken
	}
}

func (e *explorer) renamePrefix(ctx context.Context, srcPrefix, dstPrefix string) error {
	if !strings.HasSuffix(srcPrefix, "/") {
		srcPrefix += "/"
	}
	if !strings.HasSuffix(dstPrefix, "/") {
		dstPrefix += "/"
	}

	var contToken *string

	for {
		out, err := e.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(e.bucket),
			Prefix:            aws.String(srcPrefix),
			ContinuationToken: contToken,
		})
		if err != nil {
			return err
		}

		for _, obj := range out.Contents {
			oldKey := aws.ToString(obj.Key)

			// Compute relative path
			rel := strings.TrimPrefix(oldKey, srcPrefix)

			// Correct S3 key join
			newKey := dstPrefix + rel

			// Correct CopySource
			copySource := url.PathEscape(e.bucket + "/" + oldKey)

			_, err := e.client.CopyObject(ctx, &s3.CopyObjectInput{
				Bucket:     aws.String(e.bucket),
				CopySource: aws.String(copySource),
				Key:        aws.String(newKey),
			})
			if err != nil {
				return err
			}
		}

		if !aws.ToBool(out.IsTruncated) {
			break
		}
		contToken = out.NextContinuationToken
	}

	return e.deletePrefix(ctx, srcPrefix)
}

func (s *explorer) Metadata(ctx context.Context, path string) (map[string]string, error) {
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return nil, err
	}

	m := map[string]string{
		"Name":          path,
		"Bucket":        s.bucket,
		"Size":          fmt.Sprintf("%d", aws.ToInt64(out.ContentLength)),
		"Last Modified": out.LastModified.Format(time.RFC3339),
		"ETag":          aws.ToString(out.ETag),
		"Content-Type":  aws.ToString(out.ContentType),
	}

	if s := string(out.StorageClass); s != "" {
		m["Storage Class"] = s
	}

	// Add user metadata
	for k, v := range out.Metadata {
		m["x-amz-meta-"+k] = v
	}

	return m, nil
}
