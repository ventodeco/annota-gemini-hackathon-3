package testutil

import (
	"context"
	"time"

	"github.com/gemini-hackathon/app/internal/models"
)

type MockDB struct {
	users          map[int64]*models.User
	scans          map[int64]*models.Scan
	annotations    map[int64]*models.Annotation
	userByEmail    map[string]*models.User
	userByProvider map[string]*models.User
	nextUserID     int64
	nextScanID     int64
	nextAnnID      int64
}

func NewMockDB() *MockDB {
	return &MockDB{
		users:          make(map[int64]*models.User),
		scans:          make(map[int64]*models.Scan),
		annotations:    make(map[int64]*models.Annotation),
		userByEmail:    make(map[string]*models.User),
		userByProvider: make(map[string]*models.User),
		nextUserID:     1,
		nextScanID:     1,
		nextAnnID:      1,
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

func (m *MockDB) CreateAnnotation(ctx context.Context, annotation *models.Annotation) (int64, error) {
	annotation.ID = m.nextAnnID
	m.nextAnnID++
	m.annotations[annotation.ID] = annotation
	return annotation.ID, nil
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
