package service

import (
	"context"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/media"
)

type stubImageProcessor struct {
	output      []byte
	contentType string
	err         error

	calls   int
	last    media.Upload
	lastMax int
}

func (s *stubImageProcessor) Process(ctx context.Context, upload media.Upload, maxDimension int) (*media.Result, error) {
	s.calls++
	s.last = upload
	s.lastMax = maxDimension
	if s.err != nil {
		return nil, s.err
	}
	ct := s.contentType
	if ct == "" {
		ct = upload.ContentType
	}
	return &media.Result{
		Bytes:       append([]byte(nil), s.output...),
		ContentType: ct,
		Resized:     true,
	}, nil
}
