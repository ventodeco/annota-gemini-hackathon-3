package handlers

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
)

var (
	ErrMultipartParse = errors.New("multipart parse failed")
	ErrMultipartField = errors.New("multipart field missing")
	ErrUploadTooLarge = errors.New("upload too large")
)

// readFormFile parses the multipart form, reads the named file field, and returns its bytes and header.
func readFormFile(w http.ResponseWriter, r *http.Request, maxBytes int64, field string) ([]byte, *multipart.FileHeader, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxMultipartBodyBytes(maxBytes))
	if err := r.ParseMultipartForm(maxBytes); err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			return nil, nil, fmt.Errorf("%w: %v", ErrUploadTooLarge, err)
		}
		return nil, nil, fmt.Errorf("%w: %v", ErrMultipartParse, err)
	}

	file, header, err := r.FormFile(field)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrMultipartField, err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, header, fmt.Errorf("read upload: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, header, ErrUploadTooLarge
	}

	return data, header, nil
}

func maxMultipartBodyBytes(maxFileBytes int64) int64 {
	const multipartOverheadAllowance = 1 << 20
	if maxFileBytes <= 0 {
		return multipartOverheadAllowance
	}
	return maxFileBytes + multipartOverheadAllowance
}

func fileTooLargeMBMessage(maxBytes int64) string {
	return fmt.Sprintf("File too large. Maximum size is %d MB.", maxBytes/(1024*1024))
}
