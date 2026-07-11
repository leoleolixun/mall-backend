package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"go-mall/internal/config"

	"github.com/tencentyun/cos-go-sdk-v5"
)

type tencentCOS struct {
	client        *cos.Client
	publicBaseURL string
}

func newTencentCOS(cfg config.TencentCOSConfig, publicBaseURL string) (ObjectStorage, error) {
	if strings.TrimSpace(cfg.BucketURL) == "" || strings.TrimSpace(cfg.SecretID) == "" || strings.TrimSpace(cfg.SecretKey) == "" {
		return nil, fmt.Errorf("腾讯云 COS 需要配置 bucket_url、secret_id 和 secret_key")
	}

	bucketURL, err := url.Parse(strings.TrimSpace(cfg.BucketURL))
	if err != nil || bucketURL.Host == "" || (bucketURL.Scheme != "http" && bucketURL.Scheme != "https") {
		return nil, fmt.Errorf("storage.cos.bucket_url 必须是完整的存储桶 URL")
	}

	client := cos.NewClient(&cos.BaseURL{BucketURL: bucketURL}, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  strings.TrimSpace(cfg.SecretID),
			SecretKey: strings.TrimSpace(cfg.SecretKey),
		},
	})

	return &tencentCOS{client: client, publicBaseURL: publicBaseURL}, nil
}

func (s *tencentCOS) Put(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	_, err := s.client.Object.Put(ctx, key, reader, &cos.ObjectPutOptions{
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{
			ContentType:   contentType,
			ContentLength: size,
		},
	})
	if err != nil {
		return fmt.Errorf("上传到腾讯云 COS 失败: %w", err)
	}
	return nil
}

func (s *tencentCOS) Provider() string {
	return ProviderCOS
}

func (s *tencentCOS) PublicURL(key string) string {
	return buildPublicURL(s.publicBaseURL, key)
}
