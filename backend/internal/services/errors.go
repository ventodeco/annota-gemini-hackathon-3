// Package services holds the business orchestration behind the HTTP handlers:
// scan lifecycle and OCR, annotation management, and metered AI calls.
// Handlers decode requests and map these errors to responses; storage stays
// the data access layer.
package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/gemini-hackathon/app/internal/logger"
	"github.com/gemini-hackathon/app/internal/middleware"
)

var (
	ErrScanNotFound       = errors.New("scan not found")
	ErrScanForbidden      = errors.New("scan belongs to another user")
	ErrScanImageNotFound  = errors.New("scan image not found")
	ErrAnnotationNotFound = errors.New("annotation not found")
	ErrAnnotationDenied   = errors.New("annotation belongs to another user")
	ErrUserNotFound       = errors.New("user not found")

	// ErrScanCreate and ErrImageSave report which upload step failed under
	// the account lock; any other upload error is the lock itself.
	ErrScanCreate = errors.New("failed to create scan")
	ErrImageSave  = errors.New("failed to save scan image")
	// ErrStorage wraps a data access failure that is not a missing record.
	ErrStorage = errors.New("storage failure")
)

// AdmissionError is a usage admission failure (quota, budget, request size or
// meter outage). Err is the original usage error.
type AdmissionError struct {
	Err error
}

func (e *AdmissionError) Error() string { return "usage admission: " + e.Err.Error() }
func (e *AdmissionError) Unwrap() error { return e.Err }

func requestLogger(ctx context.Context, userID int64) *logger.Logger {
	return logger.GetDefaultLogger().WithRequestID(middleware.GetRequestID(ctx)).WithUserID(userID)
}

// wrapNotFound reports a failed lookup as sentinel while keeping the cause.
func wrapNotFound(sentinel, err error) error {
	return fmt.Errorf("%w: %w", sentinel, err)
}
