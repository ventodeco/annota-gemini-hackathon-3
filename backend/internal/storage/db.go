package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gemini-hackathon/app/internal/models"
)

type DB interface {
	CreateUser(ctx context.Context, user *models.User) error
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetUserByProvider(ctx context.Context, provider, providerID string) (*models.User, error)
	GetUserByID(ctx context.Context, userID int64) (*models.User, error)
	UpdateUserLanguage(ctx context.Context, userID int64, language string) error

	CreateScan(ctx context.Context, scan *models.Scan) (int64, error)
	GetScanByID(ctx context.Context, scanID int64) (*models.Scan, error)
	GetScansByUserID(ctx context.Context, userID int64, page, size int) ([]*models.Scan, error)
	UpdateScanImageURL(ctx context.Context, scanID int64, imageURL string) error
	UpdateScanOCR(ctx context.Context, scanID int64, text, language string) error

	CreateAnnotation(ctx context.Context, annotation *models.Annotation) (int64, error)
	GetAnnotationByID(ctx context.Context, annotationID int64) (*models.Annotation, error)
	GetAnnotationsByUserID(ctx context.Context, userID int64, page, size int) ([]*models.Annotation, error)
}

type postgresDB struct {
	db *sql.DB
}

func NewPostgresDB(db *sql.DB) DB {
	return &postgresDB{db: db}
}

func (s *postgresDB) CreateUser(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (email, provider, provider_id, avatar_url, preferred_language, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`
	err := s.db.QueryRowContext(ctx, query,
		user.Email,
		user.Provider,
		user.ProviderID,
		user.AvatarURL,
		user.PreferredLanguage,
		user.CreatedAt,
		user.UpdatedAt,
	).Scan(&user.ID)
	return err
}

func (s *postgresDB) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, email, provider, provider_id, avatar_url, preferred_language, created_at, updated_at
		FROM users
		WHERE email = $1
	`
	user, err := s.scanUser(s.db.QueryRowContext(ctx, query, email))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return user, err
}

func (s *postgresDB) GetUserByProvider(ctx context.Context, provider, providerID string) (*models.User, error) {
	query := `
		SELECT id, email, provider, provider_id, avatar_url, preferred_language, created_at, updated_at
		FROM users
		WHERE provider = $1 AND provider_id = $2
	`
	user, err := s.scanUser(s.db.QueryRowContext(ctx, query, provider, providerID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return user, err
}

func (s *postgresDB) GetUserByID(ctx context.Context, userID int64) (*models.User, error) {
	query := `
		SELECT id, email, provider, provider_id, avatar_url, preferred_language, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	user, err := s.scanUser(s.db.QueryRowContext(ctx, query, userID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return user, err
}

func (s *postgresDB) UpdateUserLanguage(ctx context.Context, userID int64, language string) error {
	query := `
		UPDATE users
		SET preferred_language = $1, updated_at = $2
		WHERE id = $3
	`
	_, err := s.db.ExecContext(ctx, query, language, time.Now(), userID)
	return err
}

func (s *postgresDB) scanUser(row *sql.Row) (*models.User, error) {
	var user models.User
	var avatarURL sql.NullString
	var preferredLanguage string
	var createdAt, updatedAt time.Time

	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.Provider,
		&user.ProviderID,
		&avatarURL,
		&preferredLanguage,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}

	if avatarURL.Valid {
		user.AvatarURL = &avatarURL.String
	}
	user.PreferredLanguage = preferredLanguage
	user.CreatedAt = createdAt
	user.UpdatedAt = updatedAt

	return &user, nil
}

func (s *postgresDB) CreateScan(ctx context.Context, scan *models.Scan) (int64, error) {
	query := `
		INSERT INTO scans (user_id, image_url, full_ocr_text, detected_language, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`
	err := s.db.QueryRowContext(ctx, query,
		scan.UserID,
		scan.ImageURL,
		scan.FullOCRText,
		scan.DetectedLanguage,
		scan.CreatedAt,
	).Scan(&scan.ID)
	return scan.ID, err
}

func (s *postgresDB) GetScanByID(ctx context.Context, scanID int64) (*models.Scan, error) {
	query := `
		SELECT id, user_id, image_url, full_ocr_text, detected_language, created_at
		FROM scans
		WHERE id = $1
	`
	var scan models.Scan
	var fullOCRText, detectedLanguage sql.NullString
	var createdAt time.Time

	err := s.db.QueryRowContext(ctx, query, scanID).Scan(
		&scan.ID,
		&scan.UserID,
		&scan.ImageURL,
		&fullOCRText,
		&detectedLanguage,
		&createdAt,
	)
	if err != nil {
		return nil, err
	}

	if fullOCRText.Valid {
		scan.FullOCRText = &fullOCRText.String
	}
	if detectedLanguage.Valid {
		scan.DetectedLanguage = &detectedLanguage.String
	}
	scan.CreatedAt = createdAt

	return &scan, nil
}

func (s *postgresDB) GetScansByUserID(ctx context.Context, userID int64, page, size int) ([]*models.Scan, error) {
	offset := (page - 1) * size
	query := `
		SELECT id, user_id, image_url, full_ocr_text, detected_language, created_at
		FROM scans
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := s.db.QueryContext(ctx, query, userID, size, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scans []*models.Scan
	for rows.Next() {
		var scan models.Scan
		var fullOCRText, detectedLanguage sql.NullString
		var createdAt time.Time

		err := rows.Scan(
			&scan.ID,
			&scan.UserID,
			&scan.ImageURL,
			&fullOCRText,
			&detectedLanguage,
			&createdAt,
		)
		if err != nil {
			return nil, err
		}

		if fullOCRText.Valid {
			scan.FullOCRText = &fullOCRText.String
		}
		if detectedLanguage.Valid {
			scan.DetectedLanguage = &detectedLanguage.String
		}
		scan.CreatedAt = createdAt

		scans = append(scans, &scan)
	}

	return scans, rows.Err()
}

func (s *postgresDB) UpdateScanOCR(ctx context.Context, scanID int64, text, language string) error {
	query := `
		UPDATE scans
		SET full_ocr_text = $1, detected_language = $2
		WHERE id = $3
	`
	_, err := s.db.ExecContext(ctx, query, text, language, scanID)
	return err
}

func (s *postgresDB) UpdateScanImageURL(ctx context.Context, scanID int64, imageURL string) error {
	query := `
		UPDATE scans
		SET image_url = $1
		WHERE id = $2
	`
	_, err := s.db.ExecContext(ctx, query, imageURL, scanID)
	return err
}

func (s *postgresDB) CreateAnnotation(ctx context.Context, annotation *models.Annotation) (int64, error) {
	nuanceJSON, err := json.Marshal(annotation.NuanceData)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal nuance_data: %w", err)
	}

	query := `
		INSERT INTO annotations (user_id, scan_id, highlighted_text, context_text, nuance_data, is_bookmarked, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`
	err = s.db.QueryRowContext(ctx, query,
		annotation.UserID,
		annotation.ScanID,
		annotation.HighlightedText,
		annotation.ContextText,
		nuanceJSON,
		annotation.IsBookmarked,
		annotation.CreatedAt,
	).Scan(&annotation.ID)
	return annotation.ID, err
}

func (s *postgresDB) GetAnnotationByID(ctx context.Context, annotationID int64) (*models.Annotation, error) {
	query := `
		SELECT id, user_id, scan_id, highlighted_text, context_text, nuance_data, is_bookmarked, created_at
		FROM annotations
		WHERE id = $1
	`
	var annotation models.Annotation
	var scanID sql.NullInt64
	var contextText sql.NullString
	var nuanceData []byte
	var createdAt time.Time

	err := s.db.QueryRowContext(ctx, query, annotationID).Scan(
		&annotation.ID,
		&annotation.UserID,
		&scanID,
		&annotation.HighlightedText,
		&contextText,
		&nuanceData,
		&annotation.IsBookmarked,
		&createdAt,
	)
	if err != nil {
		return nil, err
	}

	if scanID.Valid {
		annotation.ScanID = &scanID.Int64
	}
	if contextText.Valid {
		annotation.ContextText = &contextText.String
	}
	if err := json.Unmarshal(nuanceData, &annotation.NuanceData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal nuance_data: %w", err)
	}
	annotation.CreatedAt = createdAt

	return &annotation, nil
}

func (s *postgresDB) GetAnnotationsByUserID(ctx context.Context, userID int64, page, size int) ([]*models.Annotation, error) {
	offset := (page - 1) * size
	query := `
		SELECT id, user_id, scan_id, highlighted_text, context_text, nuance_data, is_bookmarked, created_at
		FROM annotations
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := s.db.QueryContext(ctx, query, userID, size, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var annotations []*models.Annotation
	for rows.Next() {
		var annotation models.Annotation
		var scanID sql.NullInt64
		var contextText sql.NullString
		var nuanceData []byte
		var createdAt time.Time

		err := rows.Scan(
			&annotation.ID,
			&annotation.UserID,
			&scanID,
			&annotation.HighlightedText,
			&contextText,
			&nuanceData,
			&annotation.IsBookmarked,
			&createdAt,
		)
		if err != nil {
			return nil, err
		}

		if scanID.Valid {
			annotation.ScanID = &scanID.Int64
		}
		if contextText.Valid {
			annotation.ContextText = &contextText.String
		}
		if err := json.Unmarshal(nuanceData, &annotation.NuanceData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal nuance_data: %w", err)
		}
		annotation.CreatedAt = createdAt

		annotations = append(annotations, &annotation)
	}

	return annotations, rows.Err()
}
