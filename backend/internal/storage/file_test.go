package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalFileStorageSaveOpenDelete(t *testing.T) {
	baseDir := t.TempDir()
	store, err := NewLocalFileStorage(baseDir)
	if err != nil {
		t.Fatalf("NewLocalFileStorage() error = %v", err)
	}

	data := []byte("image-bytes")
	path, hash, err := store.SaveImage("123", data, "image/png")
	if err != nil {
		t.Fatalf("SaveImage() error = %v", err)
	}
	if hash == nil || *hash == "" {
		t.Fatalf("SaveImage() hash = %v, want non-empty", hash)
	}
	if filepath.Ext(path) != ".png" {
		t.Fatalf("SaveImage() extension = %s, want .png", filepath.Ext(path))
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("os.Stat(%q) error = %v", path, err)
	}

	readData, err := store.OpenImage(path)
	if err != nil {
		t.Fatalf("OpenImage() error = %v", err)
	}
	if !bytes.Equal(readData, data) {
		t.Fatalf("OpenImage() data mismatch")
	}

	if err := store.DeleteImage(path); err != nil {
		t.Fatalf("DeleteImage() error = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file should be deleted, stat err = %v", err)
	}
}

func TestGetExtensionFromMimeType(t *testing.T) {
	tests := []struct {
		name     string
		mimeType string
		want     string
	}{
		{name: "jpeg", mimeType: "image/jpeg", want: ".jpg"},
		{name: "jpg", mimeType: "image/jpg", want: ".jpg"},
		{name: "png", mimeType: "image/png", want: ".png"},
		{name: "webp", mimeType: "image/webp", want: ".webp"},
		{name: "default", mimeType: "application/octet-stream", want: ".bin"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getExtensionFromMimeType(tt.mimeType)
			if got != tt.want {
				t.Fatalf("getExtensionFromMimeType(%q) = %q, want %q", tt.mimeType, got, tt.want)
			}
		})
	}
}

func TestCalculateSHA256(t *testing.T) {
	hash, err := CalculateSHA256(bytes.NewBufferString("hello"))
	if err != nil {
		t.Fatalf("CalculateSHA256() error = %v", err)
	}
	const expected = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if hash != expected {
		t.Fatalf("CalculateSHA256() = %q, want %q", hash, expected)
	}
}
