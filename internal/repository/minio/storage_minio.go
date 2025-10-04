package minio

import (
	"context"
	"fmt"
	"io"
	"strings"

	minio "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Storage struct {
	client     *minio.Client
	publicBase string
}

func NewClient(endpoint, key, secret string, useSSL bool) (*minio.Client, error) {
	return minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(key, secret, ""),
		Secure: useSSL,
	})
}

func NewStorage(client *minio.Client, publicBase string) *Storage {
	return &Storage{client: client, publicBase: strings.TrimRight(publicBase, "/")}
}

func (s *Storage) Upload(ctx context.Context, bucket, objectName, contentType string, reader io.Reader, size int64) (string, error) {
	_, err := s.client.PutObject(ctx, bucket, objectName, reader, size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return "", err
	}
	if s.publicBase != "" {
		return fmt.Sprintf("%s/%s", s.publicBase, objectName), nil
	}
	endpoint := s.client.EndpointURL()
	scheme := "https"
	if endpoint != nil {
		scheme = endpoint.Scheme
		if scheme == "" {
			scheme = "https"
		}
		host := endpoint.Host
		if host != "" {
			return fmt.Sprintf("%s://%s/%s/%s", scheme, host, bucket, objectName), nil
		}
	}
	return fmt.Sprintf("%s/%s", bucket, objectName), nil
}
