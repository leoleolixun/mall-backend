package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"go-mall/internal/config"
)

const (
	ProviderCOS   = "cos"
	ProviderQiniu = "qiniu"
)

type ObjectStorage interface {
	Put(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error
	Provider() string
	PublicURL(key string) string
}

type disabledStorage struct{}

func (disabledStorage) Put(context.Context, string, io.Reader, int64, string) error {
	return fmt.Errorf("图片存储服务尚未配置")
}

func (disabledStorage) Provider() string {
	return ""
}

func (disabledStorage) PublicURL(string) string {
	return ""
}

func New(cfg config.StorageConfig) (ObjectStorage, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider == "" {
		return disabledStorage{}, nil
	}

	publicBaseURL, err := normalizeBaseURL(cfg.PublicBaseURL)
	if err != nil {
		return nil, fmt.Errorf("storage.public_base_url 配置错误: %w", err)
	}

	switch provider {
	case ProviderCOS:
		return newTencentCOS(cfg.COS, publicBaseURL)
	case ProviderQiniu:
		return newQiniu(cfg.Qiniu, publicBaseURL)
	default:
		return nil, fmt.Errorf("不支持的图片存储服务: %s", cfg.Provider)
	}
}

func normalizeBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("必须是包含协议和域名的完整 URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("只支持 http 或 https")
	}
	return strings.TrimRight(raw, "/"), nil
}

func buildPublicURL(baseURL string, key string) string {
	return baseURL + "/" + strings.TrimLeft(key, "/")
}
