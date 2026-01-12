package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type FileStorage interface {
	SaveImage(scanID string, data []byte, mimeType string) (string, *string, error)
	OpenImage(path string) ([]byte, error)
	DeleteImage(path string) error
}

type localFileStorage struct {
	baseDir string
}

func NewLocalFileStorage(baseDir string) (FileStorage, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}
	return &localFileStorage{baseDir: baseDir}, nil
}

func (l *localFileStorage) SaveImage(scanID string, data []byte, mimeType string) (string, *string, error) {
	ext := getExtensionFromMimeType(mimeType)
	filename := fmt.Sprintf("%s%s", scanID, ext)
	path := filepath.Join(l.baseDir, filename)

	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", nil, fmt.Errorf("failed to write image file: %w", err)
	}

	hash := sha256.Sum256(data)
	hashStr := hex.EncodeToString(hash[:])

	return path, &hashStr, nil
}

func (l *localFileStorage) OpenImage(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read image file: %w", err)
	}
	return data, nil
}

func (l *localFileStorage) DeleteImage(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete image file: %w", err)
	}
	return nil
}

func getExtensionFromMimeType(mimeType string) string {
	switch mimeType {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	default:
		return ".bin"
	}
}

func CalculateSHA256(reader io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, reader); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
