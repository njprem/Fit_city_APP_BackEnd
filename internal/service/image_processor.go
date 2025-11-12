package service

import (
	"bytes"
	"context"
	"io"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/media"
)

func prepareImageForUpload(ctx context.Context, processor media.Processor, upload media.Upload, maxDimension int) (io.Reader, int64, string, error) {
	if processor == nil {
		return upload.Reader, upload.Size, upload.ContentType, nil
	}
	result, err := processor.Process(ctx, upload, maxDimension)
	if err != nil {
		return nil, 0, "", err
	}
	return bytes.NewReader(result.Bytes), int64(len(result.Bytes)), result.ContentType, nil
}
