package testutil

import (
	"context"
	"errors"

	"github.com/gemini-hackathon/app/internal/gemini"
	"github.com/gemini-hackathon/app/internal/models"
)

type MockGeminiClient struct {
	OCRFunc      func(ctx context.Context, imageData []byte, mimeType string) (*gemini.OCRResponse, error)
	AnnotateFunc func(ctx context.Context, ocrText string, selectedText string) (*gemini.AnnotationResponse, error)
}

func (m *MockGeminiClient) OCR(ctx context.Context, imageData []byte, mimeType string) (*gemini.OCRResponse, error) {
	if m.OCRFunc != nil {
		return m.OCRFunc(ctx, imageData, mimeType)
	}
	return &gemini.OCRResponse{
		RawText:        "Mock OCR text",
		StructuredJSON: `{"raw_text":"Mock OCR text","language":"ja"}`,
		Language:       "ja",
	}, nil
}

func (m *MockGeminiClient) Annotate(ctx context.Context, ocrText string, selectedText string) (*gemini.AnnotationResponse, error) {
	if m.AnnotateFunc != nil {
		return m.AnnotateFunc(ctx, ocrText, selectedText)
	}
	return &gemini.AnnotationResponse{
		Meaning:            "Mock meaning",
		UsageExample:       "Mock usage example",
		WhenToUse:          "Mock when to use",
		WordBreakdown:      "Mock word breakdown",
		AlternativeMeanings: "Mock alternative meanings",
	}, nil
}

type MockDB struct {
	CreateSessionFunc           func(ctx context.Context, session *models.Session) error
	GetSessionFunc              func(ctx context.Context, id string) (*models.Session, error)
	UpdateSessionLastSeenFunc   func(ctx context.Context, id string) error
	CreateScanFunc               func(ctx context.Context, scan *models.Scan) error
	UpdateScanStatusFunc        func(ctx context.Context, scanID string, status string) error
	GetScanFunc                  func(ctx context.Context, id string) (*models.Scan, error)
	GetScanWithOCRFunc           func(ctx context.Context, id string) (*models.Scan, *models.OCRResult, error)
	CreateScanImageFunc          func(ctx context.Context, image *models.ScanImage) error
	GetScanImageFunc             func(ctx context.Context, scanID string) (*models.ScanImage, error)
	CreateOCRResultFunc          func(ctx context.Context, result *models.OCRResult) error
	GetOCRResultFunc             func(ctx context.Context, scanID string) (*models.OCRResult, error)
	CreateAnnotationFunc         func(ctx context.Context, annotation *models.Annotation) error
	GetAnnotationsByScanIDFunc  func(ctx context.Context, scanID string) ([]*models.Annotation, error)
}

func (m *MockDB) CreateSession(ctx context.Context, session *models.Session) error {
	if m.CreateSessionFunc != nil {
		return m.CreateSessionFunc(ctx, session)
	}
	return nil
}

func (m *MockDB) GetSession(ctx context.Context, id string) (*models.Session, error) {
	if m.GetSessionFunc != nil {
		return m.GetSessionFunc(ctx, id)
	}
	return nil, errors.New("session not found")
}

func (m *MockDB) UpdateSessionLastSeen(ctx context.Context, id string) error {
	if m.UpdateSessionLastSeenFunc != nil {
		return m.UpdateSessionLastSeenFunc(ctx, id)
	}
	return nil
}

func (m *MockDB) CreateScan(ctx context.Context, scan *models.Scan) error {
	if m.CreateScanFunc != nil {
		return m.CreateScanFunc(ctx, scan)
	}
	return nil
}

func (m *MockDB) UpdateScanStatus(ctx context.Context, scanID string, status string) error {
	if m.UpdateScanStatusFunc != nil {
		return m.UpdateScanStatusFunc(ctx, scanID, status)
	}
	return nil
}

func (m *MockDB) GetScan(ctx context.Context, id string) (*models.Scan, error) {
	if m.GetScanFunc != nil {
		return m.GetScanFunc(ctx, id)
	}
	return nil, errors.New("scan not found")
}

func (m *MockDB) GetScanWithOCR(ctx context.Context, id string) (*models.Scan, *models.OCRResult, error) {
	if m.GetScanWithOCRFunc != nil {
		return m.GetScanWithOCRFunc(ctx, id)
	}
	return nil, nil, errors.New("scan not found")
}

func (m *MockDB) CreateScanImage(ctx context.Context, image *models.ScanImage) error {
	if m.CreateScanImageFunc != nil {
		return m.CreateScanImageFunc(ctx, image)
	}
	return nil
}

func (m *MockDB) GetScanImage(ctx context.Context, scanID string) (*models.ScanImage, error) {
	if m.GetScanImageFunc != nil {
		return m.GetScanImageFunc(ctx, scanID)
	}
	return nil, errors.New("scan image not found")
}

func (m *MockDB) CreateOCRResult(ctx context.Context, result *models.OCRResult) error {
	if m.CreateOCRResultFunc != nil {
		return m.CreateOCRResultFunc(ctx, result)
	}
	return nil
}

func (m *MockDB) GetOCRResult(ctx context.Context, scanID string) (*models.OCRResult, error) {
	if m.GetOCRResultFunc != nil {
		return m.GetOCRResultFunc(ctx, scanID)
	}
	return nil, errors.New("OCR result not found")
}

func (m *MockDB) CreateAnnotation(ctx context.Context, annotation *models.Annotation) error {
	if m.CreateAnnotationFunc != nil {
		return m.CreateAnnotationFunc(ctx, annotation)
	}
	return nil
}

func (m *MockDB) GetAnnotationsByScanID(ctx context.Context, scanID string) ([]*models.Annotation, error) {
	if m.GetAnnotationsByScanIDFunc != nil {
		return m.GetAnnotationsByScanIDFunc(ctx, scanID)
	}
	return nil, nil
}

type MockFileStorage struct {
	SaveImageFunc  func(scanID string, data []byte, mimeType string) (string, *string, error)
	OpenImageFunc   func(path string) ([]byte, error)
	DeleteImageFunc func(path string) error
}

func (m *MockFileStorage) SaveImage(scanID string, data []byte, mimeType string) (string, *string, error) {
	if m.SaveImageFunc != nil {
		return m.SaveImageFunc(scanID, data, mimeType)
	}
	hash := "mock-hash"
	return "/mock/path", &hash, nil
}

func (m *MockFileStorage) OpenImage(path string) ([]byte, error) {
	if m.OpenImageFunc != nil {
		return m.OpenImageFunc(path)
	}
	return []byte("mock image data"), nil
}

func (m *MockFileStorage) DeleteImage(path string) error {
	if m.DeleteImageFunc != nil {
		return m.DeleteImageFunc(path)
	}
	return nil
}
