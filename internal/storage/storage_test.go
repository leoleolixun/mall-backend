package storage

import (
	"testing"

	"go-mall/internal/config"
)

func TestNewAllowsDisabledStorage(t *testing.T) {
	client, err := New(config.StorageConfig{})
	if err != nil {
		t.Fatalf("disabled storage returned error: %v", err)
	}
	if client.Provider() != "" {
		t.Fatalf("unexpected disabled provider: %s", client.Provider())
	}
}

func TestNewRejectsUnknownProvider(t *testing.T) {
	_, err := New(config.StorageConfig{
		Provider:      "unknown",
		PublicBaseURL: "https://images.example.com",
	})
	if err == nil {
		t.Fatal("expected unknown provider error")
	}
}

func TestNewValidatesCOSConfiguration(t *testing.T) {
	_, err := New(config.StorageConfig{
		Provider:      ProviderCOS,
		PublicBaseURL: "https://images.example.com",
	})
	if err == nil {
		t.Fatal("expected missing COS configuration error")
	}
}

func TestNewValidatesQiniuConfiguration(t *testing.T) {
	_, err := New(config.StorageConfig{
		Provider:      ProviderQiniu,
		PublicBaseURL: "https://images.example.com",
	})
	if err == nil {
		t.Fatal("expected missing Qiniu configuration error")
	}
}
