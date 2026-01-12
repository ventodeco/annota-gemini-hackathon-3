package storage

import (
	"context"
	"database/sql"
	"time"

	"github.com/gemini-hackathon/app/internal/models"
)

type DB interface {
	CreateSession(ctx context.Context, session *models.Session) error
	GetSession(ctx context.Context, id string) (*models.Session, error)
	UpdateSessionLastSeen(ctx context.Context, id string) error

	CreateScan(ctx context.Context, scan *models.Scan) error
	UpdateScanStatus(ctx context.Context, scanID string, status string) error
	GetScan(ctx context.Context, id string) (*models.Scan, error)
	GetScanWithOCR(ctx context.Context, id string) (*models.Scan, *models.OCRResult, error)

	CreateScanImage(ctx context.Context, image *models.ScanImage) error
	GetScanImage(ctx context.Context, scanID string) (*models.ScanImage, error)

	CreateOCRResult(ctx context.Context, result *models.OCRResult) error
	GetOCRResult(ctx context.Context, scanID string) (*models.OCRResult, error)

	CreateAnnotation(ctx context.Context, annotation *models.Annotation) error
	GetAnnotationsByScanID(ctx context.Context, scanID string) ([]*models.Annotation, error)
}

type sqliteDB struct {
	db *sql.DB
}

func NewSQLiteDB(db *sql.DB) DB {
	return &sqliteDB{db: db}
}

func (s *sqliteDB) CreateSession(ctx context.Context, session *models.Session) error {
	query := `
		INSERT INTO sessions (id, user_id, created_at, last_seen_at)
		VALUES (?, ?, ?, ?)
	`
	_, err := s.db.ExecContext(ctx, query,
		session.ID,
		session.UserID,
		session.CreatedAt.Format(time.RFC3339),
		session.LastSeenAt.Format(time.RFC3339),
	)
	return err
}

func (s *sqliteDB) GetSession(ctx context.Context, id string) (*models.Session, error) {
	query := `
		SELECT id, user_id, created_at, last_seen_at
		FROM sessions
		WHERE id = ?
	`
	var session models.Session
	var userID sql.NullString
	var createdAtStr, lastSeenAtStr string

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&session.ID,
		&userID,
		&createdAtStr,
		&lastSeenAtStr,
	)
	if err != nil {
		return nil, err
	}

	if userID.Valid {
		session.UserID = &userID.String
	}

	createdAt, err := time.Parse(time.RFC3339, createdAtStr)
	if err != nil {
		return nil, err
	}
	session.CreatedAt = createdAt

	lastSeenAt, err := time.Parse(time.RFC3339, lastSeenAtStr)
	if err != nil {
		return nil, err
	}
	session.LastSeenAt = lastSeenAt

	return &session, nil
}

func (s *sqliteDB) UpdateSessionLastSeen(ctx context.Context, id string) error {
	query := `
		UPDATE sessions
		SET last_seen_at = ?
		WHERE id = ?
	`
	_, err := s.db.ExecContext(ctx, query, time.Now().Format(time.RFC3339), id)
	return err
}

func (s *sqliteDB) CreateScan(ctx context.Context, scan *models.Scan) error {
	query := `
		INSERT INTO scans (id, session_id, user_id, source, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.ExecContext(ctx, query,
		scan.ID,
		scan.SessionID,
		scan.UserID,
		scan.Source,
		scan.Status,
		scan.CreatedAt.Format(time.RFC3339),
	)
	return err
}

func (s *sqliteDB) GetScan(ctx context.Context, id string) (*models.Scan, error) {
	query := `
		SELECT id, session_id, user_id, source, status, created_at
		FROM scans
		WHERE id = ?
	`
	var scan models.Scan
	var userID sql.NullString
	var createdAtStr string

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&scan.ID,
		&scan.SessionID,
		&userID,
		&scan.Source,
		&scan.Status,
		&createdAtStr,
	)
	if err != nil {
		return nil, err
	}

	if userID.Valid {
		scan.UserID = &userID.String
	}

	createdAt, err := time.Parse(time.RFC3339, createdAtStr)
	if err != nil {
		return nil, err
	}
	scan.CreatedAt = createdAt

	return &scan, nil
}

func (s *sqliteDB) UpdateScanStatus(ctx context.Context, scanID string, status string) error {
	query := `
		UPDATE scans
		SET status = ?
		WHERE id = ?
	`
	_, err := s.db.ExecContext(ctx, query, status, scanID)
	return err
}

func (s *sqliteDB) GetScanWithOCR(ctx context.Context, id string) (*models.Scan, *models.OCRResult, error) {
	scan, err := s.GetScan(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	ocrResult, err := s.GetOCRResult(ctx, id)
	if err != nil {
		return scan, nil, nil
	}

	return scan, ocrResult, nil
}

func (s *sqliteDB) CreateScanImage(ctx context.Context, image *models.ScanImage) error {
	query := `
		INSERT INTO scan_images (id, scan_id, storage_path, mime_type, sha256, width, height, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.ExecContext(ctx, query,
		image.ID,
		image.ScanID,
		image.StoragePath,
		image.MimeType,
		image.SHA256,
		image.Width,
		image.Height,
		image.CreatedAt.Format(time.RFC3339),
	)
	return err
}

func (s *sqliteDB) GetScanImage(ctx context.Context, scanID string) (*models.ScanImage, error) {
	query := `
		SELECT id, scan_id, storage_path, mime_type, sha256, width, height, created_at
		FROM scan_images
		WHERE scan_id = ?
	`
	var image models.ScanImage
	var sha256 sql.NullString
	var width, height sql.NullInt64
	var createdAtStr string

	err := s.db.QueryRowContext(ctx, query, scanID).Scan(
		&image.ID,
		&image.ScanID,
		&image.StoragePath,
		&image.MimeType,
		&sha256,
		&width,
		&height,
		&createdAtStr,
	)
	if err != nil {
		return nil, err
	}

	if sha256.Valid {
		image.SHA256 = &sha256.String
	}
	if width.Valid {
		w := int(width.Int64)
		image.Width = &w
	}
	if height.Valid {
		h := int(height.Int64)
		image.Height = &h
	}

	createdAt, err := time.Parse(time.RFC3339, createdAtStr)
	if err != nil {
		return nil, err
	}
	image.CreatedAt = createdAt

	return &image, nil
}

func (s *sqliteDB) CreateOCRResult(ctx context.Context, result *models.OCRResult) error {
	query := `
		INSERT INTO ocr_results (id, scan_id, model, language, raw_text, structured_json, prompt_version, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.ExecContext(ctx, query,
		result.ID,
		result.ScanID,
		result.Model,
		result.Language,
		result.RawText,
		result.StructuredJSON,
		result.PromptVersion,
		result.CreatedAt.Format(time.RFC3339),
	)
	return err
}

func (s *sqliteDB) GetOCRResult(ctx context.Context, scanID string) (*models.OCRResult, error) {
	query := `
		SELECT id, scan_id, model, language, raw_text, structured_json, prompt_version, created_at
		FROM ocr_results
		WHERE scan_id = ?
	`
	var result models.OCRResult
	var language sql.NullString
	var structuredJSON sql.NullString
	var createdAtStr string

	err := s.db.QueryRowContext(ctx, query, scanID).Scan(
		&result.ID,
		&result.ScanID,
		&result.Model,
		&language,
		&result.RawText,
		&structuredJSON,
		&result.PromptVersion,
		&createdAtStr,
	)
	if err != nil {
		return nil, err
	}

	if language.Valid {
		result.Language = &language.String
	}
	if structuredJSON.Valid {
		result.StructuredJSON = &structuredJSON.String
	}

	createdAt, err := time.Parse(time.RFC3339, createdAtStr)
	if err != nil {
		return nil, err
	}
	result.CreatedAt = createdAt

	return &result, nil
}

func (s *sqliteDB) CreateAnnotation(ctx context.Context, annotation *models.Annotation) error {
	query := `
		INSERT INTO annotations (
			id, scan_id, ocr_result_id, selected_text, selection_start, selection_end,
			meaning, usage_example, when_to_use, word_breakdown, alternative_meanings,
			model, prompt_version, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.ExecContext(ctx, query,
		annotation.ID,
		annotation.ScanID,
		annotation.OCRResultID,
		annotation.SelectedText,
		annotation.SelectionStart,
		annotation.SelectionEnd,
		annotation.Meaning,
		annotation.UsageExample,
		annotation.WhenToUse,
		annotation.WordBreakdown,
		annotation.AlternativeMeanings,
		annotation.Model,
		annotation.PromptVersion,
		annotation.CreatedAt.Format(time.RFC3339),
	)
	return err
}

func (s *sqliteDB) GetAnnotationsByScanID(ctx context.Context, scanID string) ([]*models.Annotation, error) {
	query := `
		SELECT id, scan_id, ocr_result_id, selected_text, selection_start, selection_end,
		       meaning, usage_example, when_to_use, word_breakdown, alternative_meanings,
		       model, prompt_version, created_at
		FROM annotations
		WHERE scan_id = ?
		ORDER BY created_at ASC
	`
	rows, err := s.db.QueryContext(ctx, query, scanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var annotations []*models.Annotation
	for rows.Next() {
		var annotation models.Annotation
		var selectionStart, selectionEnd sql.NullInt64
		var createdAtStr string

		err := rows.Scan(
			&annotation.ID,
			&annotation.ScanID,
			&annotation.OCRResultID,
			&annotation.SelectedText,
			&selectionStart,
			&selectionEnd,
			&annotation.Meaning,
			&annotation.UsageExample,
			&annotation.WhenToUse,
			&annotation.WordBreakdown,
			&annotation.AlternativeMeanings,
			&annotation.Model,
			&annotation.PromptVersion,
			&createdAtStr,
		)
		if err != nil {
			return nil, err
		}

		if selectionStart.Valid {
			s := int(selectionStart.Int64)
			annotation.SelectionStart = &s
		}
		if selectionEnd.Valid {
			e := int(selectionEnd.Int64)
			annotation.SelectionEnd = &e
		}

		createdAt, err := time.Parse(time.RFC3339, createdAtStr)
		if err != nil {
			return nil, err
		}
		annotation.CreatedAt = createdAt

		annotations = append(annotations, &annotation)
	}

	return annotations, rows.Err()
}
