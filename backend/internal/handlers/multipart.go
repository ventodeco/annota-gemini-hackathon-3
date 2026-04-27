package handlers

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

var (
	ErrMultipartParse = errors.New("multipart parse failed")
	ErrMultipartField = errors.New("multipart field missing")
)

// readFormFile parses the multipart form, reads the named file field, and returns its bytes and header.
func readFormFile(r *http.Request, maxMemory int64, field string) ([]byte, *multipart.FileHeader, error) {
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrMultipartParse, err)
	}

	file, header, err := r.FormFile(field)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrMultipartField, err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, header, fmt.Errorf("read upload: %w", err)
	}

	return data, header, nil
}

func fileTooLargeMBMessage(maxBytes int64) string {
	return fmt.Sprintf("File too large. Maximum size is %d MB.", maxBytes/(1024*1024))
}
