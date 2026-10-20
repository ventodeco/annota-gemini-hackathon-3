package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gemini-hackathon/app/internal/ai"
	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/knowledge"
	"github.com/gemini-hackathon/app/internal/middleware"
	"github.com/gemini-hackathon/app/internal/models"
	"github.com/gemini-hackathon/app/internal/services"
	"github.com/gemini-hackathon/app/internal/storage"
	"github.com/gemini-hackathon/app/internal/testutil"
	"github.com/gemini-hackathon/app/internal/usage"
)

// liveContextDB rejects writes made with an expired context, as PostgreSQL does.
type liveContextDB struct {
	*testutil.MockDB
}

func (d *liveContextDB) UpdateScanStatus(ctx context.Context, scanID int64, status string, failureReason *string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return d.MockDB.UpdateScanStatus(ctx, scanID, status, failureReason)
}

// lifecycleOCRClient returns a fixed OCR result or error. With waitForDeadline
// it blocks until the job's deadline passes, like a slow provider. It records
// each call's deadline.
type lifecycleOCRClient struct {
	text            string
	err             error
	waitForDeadline bool
	calls           atomic.Int32
	deadlines       chan time.Time
}

func (c *lifecycleOCRClient) OCR(ctx context.Context, _ []byte, _ string) (*ai.OCRResponse, error) {
	c.calls.Add(1)
	if c.deadlines != nil {
		deadline, _ := ctx.Deadline()
		c.deadlines <- deadline
	}
	if c.waitForDeadline {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if c.err != nil {
		return nil, c.err
	}
	return &ai.OCRResponse{RawText: c.text, Language: "ja"}, nil
}

func (c *lifecycleOCRClient) Annotate(context.Context, string, string) (*ai.AnnotationResponse, error) {
	return nil, nil
}

func (c *lifecycleOCRClient) AnnotateWithKnowledge(context.Context, string, string, []knowledge.Entry) (*ai.AnnotationResponse, error) {
	return nil, nil
}

func (c *lifecycleOCRClient) SynthesizeSpeech(context.Context, string, string) (*ai.SpeechResponse, error) {
	return nil, nil
}

func TestProcessOCRPersistsTerminalFailure(t *testing.T) {
	tests := []struct {
		name       string
		client     *lifecycleOCRClient
		deadline   time.Duration
		wantReason string
		wantCalls  int32
	}{
		{"provider still running at the OCR deadline", &lifecycleOCRClient{waitForDeadline: true}, 20 * time.Millisecond, services.OCRFailureTimeout, 1},
		{"deadline used up before OCR started", &lifecycleOCRClient{text: "駅前"}, -time.Second, services.OCRFailureTimeout, 0},
		{"provider error", &lifecycleOCRClient{err: errors.New("upstream failed")}, time.Minute, services.OCRFailureFailed, 1},
		{"no text found", &lifecycleOCRClient{text: "  \n "}, time.Minute, services.OCRFailureNoText, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := &liveContextDB{MockDB: testutil.NewMockDB()}
			scanID, err := db.CreateScan(context.Background(), &models.Scan{UserID: 1, Status: "processing"})
			if err != nil {
				t.Fatal(err)
			}
			h := NewScanHandlers(db, nil, tt.client, &config.Config{}, provenanceTestMeter())

			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(tt.deadline))
			defer cancel()
			h.scans.ProcessOCR(ctx, nil, scanID, []byte("image"), "image/png")

			scan, err := db.GetScanByID(context.Background(), scanID)
			if err != nil {
				t.Fatal(err)
			}
			if scan.Status != "failed" || scan.FailureReason == nil || *scan.FailureReason != tt.wantReason {
				t.Fatalf("scan = status %q reason %v, want failed %q", scan.Status, scan.FailureReason, tt.wantReason)
			}
			if got := tt.client.calls.Load(); got != tt.wantCalls {
				t.Fatalf("provider calls = %d, want %d", got, tt.wantCalls)
			}
		})
	}
}

// slowSaveStorage delays SaveImage, like a slow disk under the account lock.
type slowSaveStorage struct {
	delay time.Duration
}

func (s *slowSaveStorage) SaveImage(scanID string, _ []byte, _ string) (string, *string, error) {
	time.Sleep(s.delay)
	return scanID + ".jpg", nil, nil
}
func (s *slowSaveStorage) OpenImage(string) ([]byte, error)       { return nil, nil }
func (s *slowSaveStorage) DeleteImage(string) error               { return nil }
func (s *slowSaveStorage) SavePDF(string, []byte) (string, error) { return "", nil }
func (s *slowSaveStorage) OpenPDF(string) ([]byte, error)         { return nil, nil }
func (s *slowSaveStorage) DeletePDF(string) error                 { return nil }

func TestOCRDeadlineIsAbsoluteFromScanCreation(t *testing.T) {
	db := testutil.NewMockDB()
	client := &lifecycleOCRClient{text: "駅前", deadlines: make(chan time.Time, 1)}
	h := NewScanHandlers(db, &slowSaveStorage{delay: 50 * time.Millisecond}, client, &config.Config{MaxUploadSize: 1 << 20}, provenanceTestMeter())

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="image"; filename="scan.jpg"`)
	header.Set("Content-Type", "image/jpeg")
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte("jpeg bytes"))
	writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/v1/scans", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = req.WithContext(middleware.WithUserID(req.Context(), 1))
	rec := httptest.NewRecorder()
	h.CreateScanAPI(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", rec.Code, rec.Body.String())
	}

	var deadline time.Time
	select {
	case deadline = <-client.deadlines:
	case <-time.After(5 * time.Second):
		t.Fatal("OCR was not dispatched")
	}
	scan, err := db.GetScanByID(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	// The slow save must not extend the job past the interrupted cutoff.
	if want := scan.CreatedAt.Add(services.OCRProcessingTimeout); !deadline.Equal(want) {
		t.Fatalf("OCR deadline = %v, want created_at+timeout %v", deadline, want)
	}
}

// raceReadyDB stores an OCR result just before the stale transition runs,
// as a job finishing at the same moment would.
type raceReadyDB struct {
	*testutil.MockDB
}

func (d *raceReadyDB) FailStaleProcessingScan(ctx context.Context, scanID int64, cutoff time.Time, failureReason string) (bool, error) {
	if err := d.MockDB.UpdateScanOCR(ctx, scanID, "駅前", "ja", nil); err != nil {
		return false, err
	}
	return d.MockDB.FailStaleProcessingScan(ctx, scanID, cutoff, failureReason)
}

func getScanForTest(t *testing.T, h *ScanHandlers, scanID int64) GetScanResponse {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v1/scans/"+jsonInt(scanID), nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), 1))
	rec := httptest.NewRecorder()
	h.ScanByIDAPI(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET scan status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp GetScanResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	return resp
}

func jsonInt(v int64) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func TestInterruptedOCRScanIsReportedFailed(t *testing.T) {
	db := testutil.NewMockDB()
	stale, _ := db.CreateScan(context.Background(), &models.Scan{UserID: 1, Status: "processing", CreatedAt: time.Now().Add(-services.OCRInterruptedAfter - time.Minute)})
	fresh, _ := db.CreateScan(context.Background(), &models.Scan{UserID: 1, Status: "processing", CreatedAt: time.Now()})
	h := NewScanHandlers(db, nil, &lifecycleOCRClient{}, &config.Config{DefaultPageSize: 20}, provenanceTestMeter())

	got := getScanForTest(t, h, stale)
	if got.Status != "failed" || got.FailureReason == nil || *got.FailureReason != services.OCRFailureInterrupted {
		t.Fatalf("stale scan = %q %v, want failed %q", got.Status, got.FailureReason, services.OCRFailureInterrupted)
	}
	persisted, _ := db.GetScanByID(context.Background(), stale)
	if persisted.Status != "failed" {
		t.Fatalf("interrupted state not persisted: %q", persisted.Status)
	}
	if got := getScanForTest(t, h, fresh); got.Status != "processing" {
		t.Fatalf("scan within its OCR deadline = %q, want processing", got.Status)
	}

	// The list reports the same state.
	stale2, _ := db.CreateScan(context.Background(), &models.Scan{UserID: 1, Status: "processing", CreatedAt: time.Now().Add(-services.OCRInterruptedAfter - time.Minute)})
	req := httptest.NewRequest(http.MethodGet, "/v1/scans", nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), 1))
	rec := httptest.NewRecorder()
	h.ScansAPI(rec, req)
	var list GetScansResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	for _, item := range list.Data {
		if item.ID == stale2 && item.Status != "failed" {
			t.Fatalf("listed stale scan = %q, want failed", item.Status)
		}
	}
}

func TestInterruptedOCRRecoveryKeepsConcurrentResult(t *testing.T) {
	db := &raceReadyDB{MockDB: testutil.NewMockDB()}
	scanID, _ := db.CreateScan(context.Background(), &models.Scan{UserID: 1, Status: "processing", CreatedAt: time.Now().Add(-services.OCRInterruptedAfter - time.Minute)})
	h := NewScanHandlers(db, nil, &lifecycleOCRClient{}, &config.Config{}, provenanceTestMeter())

	got := getScanForTest(t, h, scanID)
	if got.Status != "ready" || got.FullText != "駅前" {
		t.Fatalf("scan = %q %q, want the concurrent ready result", got.Status, got.FullText)
	}
}

func TestExpiredOCRDeadlineReleasesThePage(t *testing.T) {
	store := testutil.NewMemoryUsageStore()
	meter := usage.NewMeter(store, usage.Limits{usage.FeatureOCR: {User: 10, Global: 100}})
	db := testutil.NewMockDB()
	scanID, _ := db.CreateScan(context.Background(), &models.Scan{UserID: 1, Status: "processing"})
	client := &lifecycleOCRClient{text: "駅前"}
	h := NewScanHandlers(db, nil, client, &config.Config{}, meter)

	usageCtx, reservation, err := meter.Reserve(context.Background(), 1, usage.FeatureOCR, 1)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithDeadline(usageCtx, time.Now().Add(-time.Second))
	defer cancel()
	h.scans.ProcessOCR(ctx, reservation, scanID, []byte("image"), "image/png")

	if client.calls.Load() != 0 {
		t.Fatal("provider was called after the deadline")
	}
	reservations := store.Reservations()
	if len(reservations) != 1 || reservations[0].Status != storage.UsageStatusReleased {
		t.Fatalf("reservations = %+v, want one released (no provider call)", reservations)
	}
}
