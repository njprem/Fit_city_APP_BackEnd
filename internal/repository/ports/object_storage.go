package ports

import (
	"context"
	"io"
)

type ObjectStorage interface {
	Upload(ctx context.Context, bucket, objectName, contentType string, reader io.Reader, size int64) (string, error)
}
