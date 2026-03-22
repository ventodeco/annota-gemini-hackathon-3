package testutil

import (
	"context"
	"database/sql"
	"time"

	"github.com/gemini-hackathon/app/internal/models"
)

type MockDB struct {
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

func (m *MockDB) CreateScan(ctx context.Context, scan *models.Scan) (int64, error) {
	scan.ID = m.nextScanID
	m.nextScanID++
	m.scans[scan.ID] = scan
	return scan.ID, nil
}

func (m *MockDB) GetScanByID(ctx context.Context, scanID int64) (*models.Scan, error) {
	return m.scans[scanID], nil
}

func (m *MockDB) GetScansByUserID(ctx context.Context, userID int64, page, size int) ([]*models.Scan, error) {
	var result []*models.Scan
	for _, scan := range m.scans {
		if scan.UserID == userID {
			result = append(result, scan)
		}
	}
	return result, nil
}

func (m *MockDB) UpdateScanOCR(ctx context.Context, scanID int64, text, language string) error {
	if scan, ok := m.scans[scanID]; ok {
		scan.FullOCRText = &text
		scan.DetectedLanguage = &language
	}
	return nil
}

func (m *MockDB) UpdateScanImageURL(ctx context.Context, scanID int64, imageURL string) error {
	if scan, ok := m.scans[scanID]; ok {
		scan.ImageURL = imageURL
	}
	return nil
}

func (m *MockDB) DeleteScan(ctx context.Context, scanID, userID int64) error {
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

func (m *MockDB) CreateDocument(ctx context.Context, doc *models.Document) (int64, error) {
	doc.ID = m.nextDocID
	m.nextDocID++
	m.documents[doc.ID] = doc
	return doc.ID, nil
}

func (m *MockDB) UpdateDocumentFileInfo(ctx context.Context, docID int64, fileURL string, pageCount int) error {
	if doc, ok := m.documents[docID]; ok {
		doc.FileURL = fileURL
		doc.PageCount = pageCount
	}
	return nil
}

func (m *MockDB) GetDocumentByID(ctx context.Context, docID int64) (*models.Document, error) {
	return m.documents[docID], nil
}

func (m *MockDB) GetScanByDocumentPage(ctx context.Context, documentID int64, pageNumber int) (*models.Scan, error) {
	for _, scan := range m.scans {
		if scan.DocumentID != nil && *scan.DocumentID == documentID &&
			scan.PageNumber != nil && *scan.PageNumber == pageNumber {
			return scan, nil
		}
	}
	return nil, nil
}

func (m *MockDB) CreateScanFromDocument(ctx context.Context, scan *models.Scan) (int64, error) {
	scan.ID = m.nextScanID
	m.nextScanID++
	m.scans[scan.ID] = scan
	return scan.ID, nil
}
