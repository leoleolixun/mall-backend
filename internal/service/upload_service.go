package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"go-mall/internal/config"
	"go-mall/internal/dto"
	"go-mall/internal/storage"

	"github.com/google/uuid"
)

const defaultMaxImageSizeMB int64 = 10

var (
	ErrInvalidImage = errors.New("图片参数不合法")
	scenePattern    = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)
	imageExtensions = map[string]string{
		"image/jpeg": ".jpg",
		"image/png":  ".png",
		"image/webp": ".webp",
		"image/gif":  ".gif",
	}
)

type UploadImageInput struct {
	Reader       io.Reader
	Size         int64
	OriginalName string
	Scene        string
}

type UploadService interface {
	UploadImage(ctx context.Context, input UploadImageInput) (*dto.UploadImageResponse, error)
	MaxImageSize() int64
}

type uploadService struct {
	storage      storage.ObjectStorage
	maxImageSize int64
	pathPrefix   string
}

func NewUploadService(objectStorage storage.ObjectStorage, cfg config.StorageConfig) UploadService {
	maxSizeMB := cfg.MaxImageSizeMB
	if maxSizeMB <= 0 {
		maxSizeMB = defaultMaxImageSizeMB
	}
	pathPrefix := strings.Trim(strings.TrimSpace(cfg.PathPrefix), "/")
	if pathPrefix == "" {
		pathPrefix = "go-mall"
	}

	return &uploadService{
		storage:      objectStorage,
		maxImageSize: maxSizeMB * 1024 * 1024,
		pathPrefix:   pathPrefix,
	}
}

func (s *uploadService) MaxImageSize() int64 {
	return s.maxImageSize
}

func (s *uploadService) UploadImage(ctx context.Context, input UploadImageInput) (*dto.UploadImageResponse, error) {
	if input.Reader == nil || input.Size <= 0 {
		return nil, fmt.Errorf("%w: 文件不能为空", ErrInvalidImage)
	}
	if input.Size > s.maxImageSize {
		return nil, fmt.Errorf("%w: 图片不能超过 %d MB", ErrInvalidImage, s.maxImageSize/(1024*1024))
	}

	scene := strings.ToLower(strings.TrimSpace(input.Scene))
	if scene == "" {
		scene = "general"
	}
	if !scenePattern.MatchString(scene) {
		return nil, fmt.Errorf("%w: scene 只能包含小写字母、数字、下划线和短横线", ErrInvalidImage)
	}

	header := make([]byte, 512)
	n, err := io.ReadFull(input.Reader, header)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, fmt.Errorf("读取图片失败: %w", err)
	}
	header = header[:n]
	contentType := http.DetectContentType(header)
	extension, ok := imageExtensions[contentType]
	if !ok {
		return nil, fmt.Errorf("%w: 只支持 JPEG、PNG、WebP 和 GIF", ErrInvalidImage)
	}

	now := time.Now()
	key := fmt.Sprintf(
		"%s/%s/%s/%s/%s%s",
		s.pathPrefix,
		scene,
		now.Format("2006"),
		now.Format("01"),
		uuid.NewString(),
		extension,
	)
	reader := io.MultiReader(bytes.NewReader(header), input.Reader)
	if err := s.storage.Put(ctx, key, reader, input.Size, contentType); err != nil {
		return nil, err
	}

	return &dto.UploadImageResponse{
		URL:         s.storage.PublicURL(key),
		Key:         key,
		Filename:    filepath.Base(input.OriginalName),
		ContentType: contentType,
		Size:        input.Size,
		Provider:    s.storage.Provider(),
	}, nil
}
