package ports

import (
	"context"
	"io"
)

type FileObject struct {
	Key         string
	ContentType string
	Bytes       int64
	SHA256      string
}

type FileStorage interface {
	Save(ctx context.Context, keyHint, contentType string, r io.Reader, maxBytes int64) (FileObject, error)
	Delete(ctx context.Context, key string) error
	Open(ctx context.Context, key string) (FileObject, io.ReadCloser, error)
}
