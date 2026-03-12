package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func newTestFileStorage(t *testing.T) (FileStorage, string) {
	t.Helper()
	tmpDir := t.TempDir()
	fs, err := NewLocalFileStorage(tmpDir)
	if err != nil {
		t.Fatalf("NewLocalFileStorage failed: %v", err)
	}
	return fs, tmpDir
}

func TestSaveImage_CreatesFileAndHash(t *testing.T) {
	fs, tmpDir := newTestFileStorage(t)

	data := []byte("fake image data")
	path, hashPtr, err := fs.SaveImage("scan_001", data, "image/jpeg")
	if err != nil {
		t.Fatalf("SaveImage returned error: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("expected file to exist at %s", path)
	}

	// Verify path format
	expectedPath := filepath.Join(tmpDir, "scan_001.jpg")
	if path != expectedPath {
		t.Errorf("path = %q, want %q", path, expectedPath)
	}

	// Verify hash correctness
	if hashPtr == nil {
		t.Fatal("expected non-nil hash pointer")
	}
	expectedHash := sha256.Sum256(data)
	expectedHashStr := hex.EncodeToString(expectedHash[:])
	if *hashPtr != expectedHashStr {
		t.Errorf("hash = %q, want %q", *hashPtr, expectedHashStr)
	}

	// Verify content written correctly
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}
	if string(content) != string(data) {
		t.Errorf("file content mismatch")
	}
}

func TestSaveImage_MimeTypes(t *testing.T) {
	tests := []struct {
		name     string
		mimeType string
		wantExt  string
	}{
		{"jpeg", "image/jpeg", ".jpg"},
		{"jpg", "image/jpg", ".jpg"},
		{"png", "image/png", ".png"},
		{"webp", "image/webp", ".webp"},
		{"unknown", "application/octet-stream", ".bin"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs, tmpDir := newTestFileStorage(t)
			data := []byte("data")
			path, _, err := fs.SaveImage("test", data, tt.mimeType)
			if err != nil {
				t.Fatalf("SaveImage returned error: %v", err)
			}
			expectedPath := filepath.Join(tmpDir, "test"+tt.wantExt)
			if path != expectedPath {
				t.Errorf("path = %q, want %q", path, expectedPath)
			}
		})
	}
}

func TestOpenImage_ReadBack(t *testing.T) {
	fs, _ := newTestFileStorage(t)

	data := []byte("read me back")
	path, _, err := fs.SaveImage("readback", data, "image/png")
	if err != nil {
		t.Fatalf("SaveImage returned error: %v", err)
	}

	content, err := fs.OpenImage(path)
	if err != nil {
		t.Fatalf("OpenImage returned error: %v", err)
	}
	if string(content) != string(data) {
		t.Errorf("content mismatch: got %q, want %q", string(content), string(data))
	}
}

func TestOpenImage_MissingFile(t *testing.T) {
	fs, _ := newTestFileStorage(t)

	_, err := fs.OpenImage("/nonexistent/path/file.jpg")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestDeleteImage_RemovesFile(t *testing.T) {
	fs, _ := newTestFileStorage(t)

	data := []byte("delete me")
	path, _, err := fs.SaveImage("delme", data, "image/jpeg")
	if err != nil {
		t.Fatalf("SaveImage returned error: %v", err)
	}

	err = fs.DeleteImage(path)
	if err != nil {
		t.Fatalf("DeleteImage returned error: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected file to be deleted")
	}
}

func TestDeleteImage_NonExistentFile(t *testing.T) {
	fs, _ := newTestFileStorage(t)

	err := fs.DeleteImage("/nonexistent/path/file.jpg")
	if err != nil {
		t.Errorf("DeleteImage on non-existent file should not error, got: %v", err)
	}
}

func TestGetExtensionFromMimeType(t *testing.T) {
	tests := []struct {
		mimeType string
		want     string
	}{
		{"image/jpeg", ".jpg"},
		{"image/jpg", ".jpg"},
		{"image/png", ".png"},
		{"image/webp", ".webp"},
		{"image/gif", ".bin"},
		{"text/plain", ".bin"},
		{"", ".bin"},
	}

	for _, tt := range tests {
		t.Run(tt.mimeType, func(t *testing.T) {
			got := getExtensionFromMimeType(tt.mimeType)
			if got != tt.want {
				t.Errorf("getExtensionFromMimeType(%q) = %q, want %q", tt.mimeType, got, tt.want)
			}
		})
	}
}
