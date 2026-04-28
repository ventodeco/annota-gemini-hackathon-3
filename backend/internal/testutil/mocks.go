package testutil

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/gemini-hackathon/app/internal/models"
)

type MockDB struct {
	mu             sync.RWMutex
	users          map[int64]*models.User
	scans          map[int64]*models.Scan
	annotations    map[int64]*models.Annotation
	documents      map[int64]*models.Document
	userByEmail    map[string]*models.User
	userByProvider map[string]*models.User
	nextUserID     int64
	nextScanID     int64
	nextAnnID      int64
	nextDocID      int64
}

func NewMockDB() *MockDB {
	return &MockDB{
		users:          make(map[int64]*models.User),
		scans:          make(map[int64]*models.Scan),
		annotations:    make(map[int64]*models.Annotation),
		documents:      make(map[int64]*models.Document),
		userByEmail:    make(map[string]*models.User),
		userByProvider: make(map[string]*models.User),
		nextUserID:     1,
		nextScanID:     1,
		nextAnnID:      1,
		nextDocID:      1,
	}
}

func (m *MockDB) CreateUser(ctx context.Context, user *models.User) error {
	user.ID = m.nextUserID
	m.nextUserID++
	m.users[user.ID] = user
	m.userByEmail[user.Email] = user
	m.userByProvider[user.Provider+":"+user.ProviderID] = user
	return nil
}

func (m *MockDB) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	return m.userByEmail[email], nil
}

func (m *MockDB) GetUserByProvider(ctx context.Context, provider, providerID string) (*models.User, error) {
	return m.userByProvider[provider+":"+providerID], nil
}

func (m *MockDB) GetUserByID(ctx context.Context, userID int64) (*models.User, error) {
	return m.users[userID], nil
}

func (m *MockDB) UpdateUserLanguage(ctx context.Context, userID int64, language string) error {
	if user, ok := m.users[userID]; ok {
		user.PreferredLanguage = language
		user.UpdatedAt = time.Now()
	}
	return nil
}

func (m *MockDB) DeleteUser(ctx context.Context, userID int64) error {
	if _, ok := m.users[userID]; !ok {
		return sql.ErrNoRows
	}
	delete(m.users, userID)
	for id, scan := range m.scans {
		if scan.UserID == userID {
			delete(m.scans, id)
		}
	}
	for id, annotation := range m.annotations {
		if annotation.UserID == userID {
			delete(m.annotations, id)
		}
	}
	for id, document := range m.documents {
		if document.UserID == userID {
			delete(m.documents, id)
		}
	}
	return nil
}

func (m *MockDB) CreateScan(ctx context.Context, scan *models.Scan) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if scan.SourceType == "" {
		scan.SourceType = "image"
	}
	if scan.Status == "" {
		scan.Status = "processing"
	}
	scan.ID = m.nextScanID
	m.nextScanID++
	m.scans[scan.ID] = scan
	return scan.ID, nil
}

func (m *MockDB) GetScanByID(ctx context.Context, scanID int64) (*models.Scan, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	scan, ok := m.scans[scanID]
	if !ok {
		return nil, nil
	}
	scanCopy := *scan
	return &scanCopy, nil
}

func (m *MockDB) GetScansByUserID(ctx context.Context, userID int64, page, size int) ([]*models.Scan, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*models.Scan
	for _, scan := range m.scans {
		if scan.UserID == userID {
			scanCopy := *scan
			result = append(result, &scanCopy)
		}
	}
	return result, nil
}

func (m *MockDB) UpdateScanOCR(ctx context.Context, scanID int64, text, language string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if scan, ok := m.scans[scanID]; ok {
		scan.FullOCRText = &text
		scan.DetectedLanguage = &language
		scan.Status = "ready"
		scan.FailureReason = nil
	}
	return nil
}

func (m *MockDB) UpdateScanStatus(ctx context.Context, scanID int64, status string, failureReason *string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if scan, ok := m.scans[scanID]; ok {
		scan.Status = status
		scan.FailureReason = failureReason
	}
	return nil
}

func (m *MockDB) UpdateScanImageURL(ctx context.Context, scanID int64, imageURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if scan, ok := m.scans[scanID]; ok {
		scan.ImageURL = imageURL
	}
	return nil
}

func (m *MockDB) DeleteScan(ctx context.Context, scanID, userID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if scan, ok := m.scans[scanID]; ok && scan.UserID == userID {
		delete(m.scans, scanID)
		return nil
	}
	return sql.ErrNoRows
}

func (m *MockDB) CreateAnnotation(ctx context.Context, annotation *models.Annotation) (int64, error) {
	annotation.ID = m.nextAnnID
	m.nextAnnID++
	m.annotations[annotation.ID] = annotation
	return annotation.ID, nil
}

func (m *MockDB) GetAnnotationByID(ctx context.Context, annotationID int64) (*models.Annotation, error) {
	return m.annotations[annotationID], nil
}

func (m *MockDB) GetAnnotationsByUserID(ctx context.Context, userID int64, page, size int) ([]*models.Annotation, error) {
	var result []*models.Annotation
	for _, ann := range m.annotations {
		if ann.UserID == userID {
			result = append(result, ann)
		}
	}
	return result, nil
}

func (m *MockDB) DeleteAnnotation(ctx context.Context, annotationID, userID int64) error {
	if ann, ok := m.annotations[annotationID]; ok && ann.UserID == userID {
		delete(m.annotations, annotationID)
		return nil
	}
	return sql.ErrNoRows
}

func (m *MockDB) GetAnnotationsByUserIDAndScanID(
	ctx context.Context,
	userID, scanID int64,
	page, size int,
) ([]*models.Annotation, error) {
	var result []*models.Annotation
	for _, ann := range m.annotations {
		if ann.UserID != userID || ann.ScanID == nil {
			continue
		}
		if *ann.ScanID == scanID {
			result = append(result, ann)
		}
	}
	return result, nil
}

func (m *MockDB) GetAnnotationsByUserIDAndDocumentPage(
	ctx context.Context,
	userID, documentID int64,
	pageNumber *int,
	page, size int,
) ([]*models.Annotation, error) {
	var result []*models.Annotation
	for _, ann := range m.annotations {
		if ann.UserID != userID || ann.DocumentID == nil || *ann.DocumentID != documentID {
			continue
		}
		if pageNumber != nil {
			if ann.PageNumber == nil || *ann.PageNumber != *pageNumber {
				continue
			}
		}
		result = append(result, ann)
	}
	return result, nil
}

func (m *MockDB) CreateDocument(ctx context.Context, doc *models.Document) (int64, error) {
	if doc.LastPageNumber <= 0 {
		doc.LastPageNumber = 1
	}
	if doc.UpdatedAt.IsZero() {
		doc.UpdatedAt = doc.CreatedAt
	}
	doc.ID = m.nextDocID
	m.nextDocID++
	m.documents[doc.ID] = doc
	return doc.ID, nil
}

func (m *MockDB) UpdateDocumentFileInfo(ctx context.Context, docID int64, fileURL string, pageCount int) error {
	if doc, ok := m.documents[docID]; ok {
		doc.FileURL = fileURL
		doc.PageCount = pageCount
		doc.UpdatedAt = time.Now()
	}
	return nil
}

func (m *MockDB) GetDocumentByID(ctx context.Context, docID int64) (*models.Document, error) {
	return m.documents[docID], nil
}

func (m *MockDB) GetDocumentsByUserID(ctx context.Context, userID int64, page, size int) ([]*models.Document, error) {
	var result []*models.Document
	for _, doc := range m.documents {
		if doc.UserID == userID {
			result = append(result, doc)
		}
	}
	return result, nil
}

func (m *MockDB) UpdateDocumentProgress(ctx context.Context, docID, userID int64, pageNumber int) error {
	if doc, ok := m.documents[docID]; ok && doc.UserID == userID {
		now := time.Now()
		doc.LastPageNumber = pageNumber
		doc.LastOpenedAt = &now
		doc.UpdatedAt = now
		return nil
	}
	return sql.ErrNoRows
}

func (m *MockDB) DeleteScansByDocument(ctx context.Context, docID, userID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, scan := range m.scans {
		if scan.UserID == userID && scan.DocumentID != nil && *scan.DocumentID == docID {
			delete(m.scans, id)
		}
	}
	return nil
}

func (m *MockDB) DeleteDocument(ctx context.Context, docID, userID int64) error {
	if doc, ok := m.documents[docID]; ok && doc.UserID == userID {
		delete(m.documents, docID)
		return nil
	}
	return sql.ErrNoRows
}

func (m *MockDB) GetScanByDocumentPage(ctx context.Context, documentID int64, pageNumber int) (*models.Scan, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, scan := range m.scans {
		if scan.DocumentID != nil && *scan.DocumentID == documentID &&
			scan.PageNumber != nil && *scan.PageNumber == pageNumber {
			scanCopy := *scan
			return &scanCopy, nil
		}
	}
	return nil, nil
}

func (m *MockDB) CreateScanFromDocument(ctx context.Context, scan *models.Scan) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if scan.SourceType == "" {
		scan.SourceType = "pdf"
	}
	if scan.Status == "" {
		scan.Status = "ready"
	}
	scan.ID = m.nextScanID
	m.nextScanID++
	m.scans[scan.ID] = scan
	return scan.ID, nil
}
