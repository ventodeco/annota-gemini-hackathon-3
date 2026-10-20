package services

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gemini-hackathon/app/internal/ai"
	"github.com/gemini-hackathon/app/internal/logger"
	"github.com/gemini-hackathon/app/internal/models"
	"github.com/gemini-hackathon/app/internal/storage"
	"github.com/gemini-hackathon/app/internal/usage"
)

// OCRProcessingTimeout is the OCR deadline, measured from the scan's creation.
const OCRProcessingTimeout = 5 * time.Minute

// OCRReservationStaleAfter is when an OCR reservation still unsettled counts
// as abandoned (its process died mid-OCR) and is refunded to the user.
const OCRReservationStaleAfter = OCRProcessingTimeout + 10*time.Minute

// ocrSettleMargin bounds the OCR result write and the detached settle that
// follow the OCR context's deadline.
const ocrSettleMargin = time.Minute

// ocrDispatchWindow is the latest age, measured from admission, at which OCR
// may still start. A job dispatched within it reaches its OCR deadline and
// settles before OCRReservationStaleAfter, so expiry never refunds live work.
// Slow pre-dispatch steps (account lock, CreateScan, SaveImage) past it get
// no OCR and an immediate release instead.
const ocrDispatchWindow = OCRReservationStaleAfter - OCRProcessingTimeout - ocrSettleMargin

// ocrStatusWriteTimeout bounds the final status write, which must still run
// after the OCR deadline has expired.
const ocrStatusWriteTimeout = 10 * time.Second

// OCRInterruptedAfter is when a scan still processing can no longer have a
// running job: the job's deadline plus time for its final write. Such a scan
// was interrupted, for example by a restart, and is reported failed.
const OCRInterruptedAfter = OCRProcessingTimeout + time.Minute

// OCR failure reasons returned in failureReason.
const (
	OCRFailureFailed      = "ocr_failed"
	OCRFailureTimeout     = "ocr_timeout"
	OCRFailureInterrupted = "ocr_interrupted"
	OCRFailureNoText      = "no_text_found"
)

// errOCRIncomplete settles an OCR reservation whose job ended without a result.
var errOCRIncomplete = errors.New("ocr job ended without a result")

// ScanService owns the image scan lifecycle: admission, storage under the
// account lock, OCR, interrupted-job recovery, and deletion.
type ScanService struct {
	db        storage.DB
	files     storage.FileStorage
	aiClient  ai.Client
	meter     *usage.Meter
	uploadDir string
}

func NewScanService(db storage.DB, files storage.FileStorage, aiClient ai.Client, meter *usage.Meter, uploadDir string) *ScanService {
	return &ScanService{db: db, files: files, aiClient: aiClient, meter: meter, uploadDir: uploadDir}
}

// CreatedScan is the state of a newly uploaded image scan.
type CreatedScan struct {
	ID            int64
	ImageURL      string
	Status        string
	FailureReason *string
}

// CreateImageScan admits one OCR page, stores the scan and its image under the
// shared account lock, and starts OCR in the background. Admission failures
// are returned as *AdmissionError; a failed upload step as ErrScanCreate or
// ErrImageSave; anything else is the account lock failing. The page is
// released whenever no OCR will run for it.
func (s *ScanService) CreateImageScan(ctx context.Context, userID int64, imageData []byte, mimeType string) (*CreatedScan, error) {
	log := requestLogger(ctx, userID)

	// Admit the OCR page before creating the scan row; refund it if no OCR starts.
	usageCtx, reservation, err := s.meter.Reserve(ctx, userID, usage.FeatureOCR, 1)
	if err != nil {
		return nil, &AdmissionError{Err: err}
	}

	now := time.Now()
	scan := &models.Scan{
		UserID:     userID,
		ImageURL:   "",
		SourceType: "image",
		Status:     "processing",
		CreatedAt:  now,
	}

	var scanID int64
	err = s.db.WithAccountLock(ctx, userID, false, func(ctx context.Context) error {
		var err error
		scanID, err = s.db.CreateScan(ctx, scan)
		if err != nil {
			log.ErrorWithErr(err, "Failed to create scan in database")
			return ErrScanCreate
		}

		if _, _, err := s.files.SaveImage(strconv.FormatInt(scanID, 10), imageData, mimeType); err != nil {
			log.ErrorWithErr(err, "Failed to save image to storage")
			s.discardFailedScan(ctx, log, &models.Scan{ID: scanID, UserID: userID})
			return ErrImageSave
		}

		if err := s.db.UpdateScanImageURL(ctx, scanID, PrivateScanImageURL(scanID)); err != nil {
			log.ErrorWithErr(err, "Failed to update scan with image URL")
		}
		return nil
	})
	if err != nil {
		if !errors.Is(err, ErrScanCreate) && !errors.Is(err, ErrImageSave) {
			log.ErrorWithErr(err, "Failed to lock account")
		}
		// No scan row or file exists, so no OCR will run: refund the page.
		s.meter.Release(ctx, reservation)
		return nil, err
	}
	imageURL := PrivateScanImageURL(scanID)

	if age := s.meter.Age(reservation); age > ocrDispatchWindow {
		// Too late to start OCR and still settle before the reservation
		// expires; fail the scan and refund instead of dispatching.
		log.Warnf("Skipping OCR dispatch %s after admission (window %s)", age, ocrDispatchWindow)
		reason := OCRFailureFailed
		if err := s.db.UpdateScanStatus(ctx, scanID, "failed", &reason); err != nil {
			log.ErrorWithErr(err, "Failed to persist skipped OCR dispatch")
		}
		s.meter.Release(ctx, reservation)
		return &CreatedScan{ID: scanID, ImageURL: imageURL, Status: "failed", FailureReason: &reason}, nil
	}

	log.WithFields(map[string]any{
		"scan_id":   scanID,
		"user_id":   userID,
		"image_url": imageURL,
	}).Infof("Scan created successfully, starting OCR processing")

	// The deadline is absolute from the scan's created_at, so a slow upload
	// can't push OCR work past the point where the scan reads as interrupted.
	ocrCtx, cancelOCR := context.WithDeadline(context.WithoutCancel(usageCtx), now.Add(OCRProcessingTimeout))
	go func() {
		defer cancelOCR()
		s.ProcessOCR(ocrCtx, reservation, scanID, imageData, mimeType)
	}()

	return &CreatedScan{ID: scanID, ImageURL: imageURL, Status: "processing"}, nil
}

// ProcessOCR runs OCR for a stored scan, persists the result or a terminal
// failure reason, and settles reservation exactly once. It never retries.
func (s *ScanService) ProcessOCR(ctx context.Context, reservation *usage.Reservation, scanID int64, imageData []byte, mimeType string) {
	log := logger.GetDefaultLogger().WithField("scan_id", scanID)

	// Settle exactly once, even on an early return or panic.
	outcome := errOCRIncomplete
	defer func() { s.meter.Finish(ctx, reservation, outcome) }()

	// Each final write gets its own short deadline, started when the write
	// starts: after a provider timeout ctx has already expired, and the scan
	// would otherwise stay processing forever.
	writeContext := func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(context.WithoutCancel(ctx), ocrStatusWriteTimeout)
	}
	markFailed := func(reason string) {
		writeCtx, cancel := writeContext()
		defer cancel()
		if err := s.db.UpdateScanStatus(writeCtx, scanID, "failed", &reason); err != nil {
			log.ErrorWithErr(err, "Failed to persist OCR failure")
		}
	}

	// The upload can use up the whole deadline; then no provider call is
	// made, so the page is released for both the user and the global budget.
	// Release settles first, which makes the deferred Finish a no-op.
	if ctx.Err() != nil {
		log.Warn("OCR deadline passed before processing started")
		s.meter.Release(ctx, reservation)
		markFailed(OCRFailureTimeout)
		return
	}

	log.Infof("Starting OCR processing: image_size=%d bytes, mime_type=%s", len(imageData), mimeType)
	ocrResp, err := s.aiClient.OCR(ctx, imageData, mimeType)
	if err != nil {
		outcome = err
		log.ErrorWithErr(err, "OCR processing failed")
		if errors.Is(err, context.DeadlineExceeded) {
			markFailed(OCRFailureTimeout)
		} else {
			markFailed(OCRFailureFailed)
		}
		return
	}
	log.Infof("OCR completed successfully: language=%s, text_length=%d", ocrResp.Language, len(ocrResp.RawText))

	if strings.TrimSpace(ocrResp.RawText) == "" {
		// The provider did the work, so the page stays charged.
		outcome = nil
		markFailed(OCRFailureNoText)
		return
	}

	writeCtx, cancelWrite := writeContext()
	defer cancelWrite()
	if err := s.db.UpdateScanOCR(writeCtx, scanID, ocrResp.RawText, ocrResp.Language, ProvenanceFor(s.aiClient, "ocr", usage.Calls(ctx))); err != nil {
		outcome = err
		log.ErrorWithErr(err, "Failed to update scan OCR in database")
		markFailed(OCRFailureFailed)
		return
	}
	outcome = nil

	log.Infof("OCR results saved to database successfully")
}

// GetScan returns the user's scan, reporting an interrupted OCR job as failed.
func (s *ScanService) GetScan(ctx context.Context, userID, scanID int64) (*models.Scan, error) {
	scan, err := s.ownedScan(ctx, userID, scanID)
	if err != nil {
		return nil, err
	}
	return s.recoverInterruptedScan(ctx, scan, requestLogger(ctx, userID)), nil
}

// ListScans returns a page of the user's scans, reporting interrupted OCR
// jobs as failed.
func (s *ScanService) ListScans(ctx context.Context, userID int64, page, size int) ([]*models.Scan, error) {
	scans, err := s.db.GetScansByUserID(ctx, userID, page, size)
	if err != nil {
		return nil, err
	}
	log := requestLogger(ctx, userID)
	for i, scan := range scans {
		scans[i] = s.recoverInterruptedScan(ctx, scan, log)
	}
	return scans, nil
}

// ScanImage returns the stored image of the user's scan and its content type.
func (s *ScanService) ScanImage(ctx context.Context, userID, scanID int64) ([]byte, string, error) {
	scan, err := s.ownedScan(ctx, userID, scanID)
	if err != nil {
		return nil, "", err
	}
	for _, candidate := range scanImagePathCandidates(s.uploadDir, scan) {
		data, err := s.files.OpenImage(candidate.path)
		if err == nil {
			return data, candidate.contentType, nil
		}
	}
	return nil, "", ErrScanImageNotFound
}

// DeleteScan removes the user's scan image and then its row under the
// exclusive account lock. The file goes first and the row is kept on failure:
// once the row is gone nothing points at the image any more, so a retry could
// never remove it. The exclusive lock keeps an in-flight upload of this scan
// from writing its file after the row is gone.
func (s *ScanService) DeleteScan(ctx context.Context, userID, scanID int64) error {
	scan, err := s.ownedScan(ctx, userID, scanID)
	if err != nil {
		return err
	}
	log := requestLogger(ctx, userID).WithField("scan_id", scanID)
	return s.db.WithAccountLock(ctx, userID, true, func(ctx context.Context) error {
		if err := s.deleteStoredScanImage(scan); err != nil {
			log.ErrorWithErr(err, "Failed to delete scan image")
			return err
		}
		if err := s.db.DeleteScan(ctx, scanID, userID); err != nil {
			log.ErrorWithErr(err, "Failed to delete scan")
			return err
		}
		return nil
	})
}

// ownedScan loads a scan, returning ErrScanNotFound (wrapping any lookup
// error) or ErrScanForbidden when it belongs to another user.
func (s *ScanService) ownedScan(ctx context.Context, userID, scanID int64) (*models.Scan, error) {
	scan, err := s.db.GetScanByID(ctx, scanID)
	if err != nil {
		return nil, wrapNotFound(ErrScanNotFound, err)
	}
	if scan == nil {
		return nil, ErrScanNotFound
	}
	if scan.UserID != userID {
		return nil, ErrScanForbidden
	}
	return scan, nil
}

// discardFailedScan removes the row and any partially written image of a scan
// whose upload failed, so no unusable record or unreachable file is left.
func (s *ScanService) discardFailedScan(ctx context.Context, log *logger.Logger, scan *models.Scan) {
	if err := s.deleteStoredScanImage(scan); err != nil {
		log.ErrorWithErr(err, "Failed to remove image of failed scan upload")
		return
	}
	if err := s.db.DeleteScan(ctx, scan.ID, scan.UserID); err != nil {
		log.ErrorWithErr(err, "Failed to remove record of failed scan upload")
	}
}

func (s *ScanService) deleteStoredScanImage(scan *models.Scan) error {
	var lastErr error
	for _, candidate := range scanImagePathCandidates(s.uploadDir, scan) {
		if err := s.files.DeleteImage(candidate.path); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// recoverInterruptedScan reports an image scan whose OCR job can no longer be
// running as failed, persisting the change. The transition is conditional, so
// an OCR result that landed concurrently wins and is returned instead.
func (s *ScanService) recoverInterruptedScan(ctx context.Context, scan *models.Scan, log *logger.Logger) *models.Scan {
	if StatusOrDefault(scan) != "processing" || scan.DocumentID != nil {
		return scan
	}
	cutoff := time.Now().Add(-OCRInterruptedAfter)
	if !scan.CreatedAt.Before(cutoff) {
		return scan
	}

	changed, err := s.db.FailStaleProcessingScan(ctx, scan.ID, cutoff, OCRFailureInterrupted)
	if err != nil {
		log.WithField("scan_id", scan.ID).ErrorWithErr(err, "Failed to mark interrupted OCR scan")
		return scan
	}
	if changed {
		recovered := *scan
		reason := OCRFailureInterrupted
		recovered.Status = "failed"
		recovered.FailureReason = &reason
		log.WithField("scan_id", scan.ID).Warn("Marked interrupted OCR scan as failed")
		return &recovered
	}

	latest, err := s.db.GetScanByID(ctx, scan.ID)
	if err != nil || latest == nil {
		return scan
	}
	return latest
}

type scanImagePathCandidate struct {
	path        string
	contentType string
}

func scanImagePathCandidates(uploadDir string, scan *models.Scan) []scanImagePathCandidate {
	if scan.ImageURL != "" && strings.HasPrefix(scan.ImageURL, "/uploads/") {
		path := filepath.Join(uploadDir, filepath.Base(scan.ImageURL))
		return []scanImagePathCandidate{{path: path, contentType: contentTypeFromImagePath(path)}}
	}

	base := filepath.Join(uploadDir, strconv.FormatInt(scan.ID, 10))
	return []scanImagePathCandidate{
		{path: base + ".jpg", contentType: "image/jpeg"},
		{path: base + ".png", contentType: "image/png"},
		{path: base + ".webp", contentType: "image/webp"},
	}
}

// PrivateScanImageURL is the authenticated API path of a scan's image.
func PrivateScanImageURL(scanID int64) string {
	return fmt.Sprintf("/v1/scans/%d/image", scanID)
}

func contentTypeFromImagePath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

// SourceTypeOrDefault infers the source type of scans saved before it was stored.
func SourceTypeOrDefault(scan *models.Scan) string {
	if scan.SourceType != "" {
		return scan.SourceType
	}
	if scan.DocumentID != nil {
		return "pdf"
	}
	return "image"
}

// StatusOrDefault infers the status of scans saved before it was stored.
func StatusOrDefault(scan *models.Scan) string {
	if scan.Status != "" {
		return scan.Status
	}
	if scan.FullOCRText != nil && strings.TrimSpace(*scan.FullOCRText) != "" {
		return "ready"
	}
	return "processing"
}
