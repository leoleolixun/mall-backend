package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"go-mall/internal/config"
)

type fakeObjectStorage struct {
	key         string
	contentType string
	size        int64
	data        []byte
}

func (s *fakeObjectStorage) Put(_ context.Context, key string, reader io.Reader, size int64, contentType string) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	s.key = key
	s.contentType = contentType
	s.size = size
	s.data = data
	return nil
}

func (s *fakeObjectStorage) Provider() string {
	return "fake"
}

func (s *fakeObjectStorage) PublicURL(key string) string {
	return "https://images.example.com/" + key
}

func TestUploadImage(t *testing.T) {
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52}
	objectStorage := &fakeObjectStorage{}
	service := NewUploadService(objectStorage, config.StorageConfig{
		MaxImageSizeMB: 2,
		PathPrefix:     "mall-assets",
	})

	result, err := service.UploadImage(context.Background(), UploadImageInput{
		Reader:       bytes.NewReader(png),
		Size:         int64(len(png)),
		OriginalName: "../avatar.png",
		Scene:        "avatar",
	})
	if err != nil {
		t.Fatalf("upload image returned error: %v", err)
	}
	if !strings.HasPrefix(result.Key, "mall-assets/avatar/") || !strings.HasSuffix(result.Key, ".png") {
		t.Fatalf("unexpected object key: %s", result.Key)
	}
	if result.Filename != "avatar.png" {
		t.Fatalf("unexpected filename: %s", result.Filename)
	}
	if result.ContentType != "image/png" || result.Provider != "fake" {
		t.Fatalf("unexpected response: %+v", result)
	}
	if result.URL != "https://images.example.com/"+result.Key {
		t.Fatalf("unexpected public url: %s", result.URL)
	}
	if !bytes.Equal(objectStorage.data, png) || objectStorage.size != int64(len(png)) {
		t.Fatalf("uploaded content does not match input")
	}
}

func TestUploadImageRejectsNonImage(t *testing.T) {
	service := NewUploadService(&fakeObjectStorage{}, config.StorageConfig{})
	data := []byte("this is not an image")

	_, err := service.UploadImage(context.Background(), UploadImageInput{
		Reader:       bytes.NewReader(data),
		Size:         int64(len(data)),
		OriginalName: "fake.jpg",
		Scene:        "avatar",
	})
	if err == nil || !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("expected invalid image error, got %v", err)
	}
}

func TestUploadImageRejectsOversizedFile(t *testing.T) {
	service := NewUploadService(&fakeObjectStorage{}, config.StorageConfig{MaxImageSizeMB: 1})

	_, err := service.UploadImage(context.Background(), UploadImageInput{
		Reader:       bytes.NewReader([]byte("unused")),
		Size:         1024*1024 + 1,
		OriginalName: "large.png",
		Scene:        "product",
	})
	if err == nil || !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("expected oversized image error, got %v", err)
	}
}

func TestUploadImageRejectsInvalidScene(t *testing.T) {
	service := NewUploadService(&fakeObjectStorage{}, config.StorageConfig{})
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}

	_, err := service.UploadImage(context.Background(), UploadImageInput{
		Reader:       bytes.NewReader(png),
		Size:         int64(len(png)),
		OriginalName: "avatar.png",
		Scene:        "../avatar",
	})
	if err == nil || !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("expected invalid scene error, got %v", err)
	}
}
