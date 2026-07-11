package storage

import (
	"context"
	"fmt"
	"io"
	"strings"

	"go-mall/internal/config"

	"github.com/qiniu/go-sdk/v7/storagev2/credentials"
	qiniuhttp "github.com/qiniu/go-sdk/v7/storagev2/http_client"
	"github.com/qiniu/go-sdk/v7/storagev2/uploader"
)

type qiniuStorage struct {
	bucket        string
	uploadManager *uploader.UploadManager
	publicBaseURL string
}

func newQiniu(cfg config.QiniuStorageConfig, publicBaseURL string) (ObjectStorage, error) {
	if strings.TrimSpace(cfg.Bucket) == "" || strings.TrimSpace(cfg.AccessKey) == "" || strings.TrimSpace(cfg.SecretKey) == "" {
		return nil, fmt.Errorf("七牛 Kodo 需要配置 bucket、access_key 和 secret_key")
	}

	credential := credentials.NewCredentials(
		strings.TrimSpace(cfg.AccessKey),
		strings.TrimSpace(cfg.SecretKey),
	)
	uploadManager := uploader.NewUploadManager(&uploader.UploadManagerOptions{
		Options: qiniuhttp.Options{Credentials: credential},
	})

	return &qiniuStorage{
		bucket:        strings.TrimSpace(cfg.Bucket),
		uploadManager: uploadManager,
		publicBaseURL: publicBaseURL,
	}, nil
}

func (s *qiniuStorage) Put(ctx context.Context, key string, reader io.Reader, _ int64, contentType string) error {
	if err := s.uploadManager.UploadReader(ctx, reader, &uploader.ObjectOptions{
		BucketName:  s.bucket,
		ObjectName:  &key,
		FileName:    key,
		ContentType: contentType,
	}, nil); err != nil {
		return fmt.Errorf("上传到七牛 Kodo 失败: %w", err)
	}
	return nil
}

func (s *qiniuStorage) Provider() string {
	return ProviderQiniu
}

func (s *qiniuStorage) PublicURL(key string) string {
	return buildPublicURL(s.publicBaseURL, key)
}
