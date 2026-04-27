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
	DeleteUser(ctx context.Context, userID int64) error

	CreateScan(ctx context.Context, scan *models.Scan) (int64, error)
	GetScanByID(ctx context.Context, scanID int64) (*models.Scan, error)
	GetScansByUserID(ctx context.Context, userID int64, page, size int) ([]*models.Scan, error)
	UpdateScanImageURL(ctx context.Context, scanID int64, imageURL string) error
	UpdateScanOCR(ctx context.Context, scanID int64, text, language string) error
	UpdateScanStatus(ctx context.Context, scanID int64, status string, failureReason *string) error
	DeleteScan(ctx context.Context, scanID, userID int64) error

	CreateDocument(ctx context.Context, doc *models.Document) (int64, error)
	UpdateDocumentFileInfo(ctx context.Context, docID int64, fileURL string, pageCount int) error
	GetDocumentByID(ctx context.Context, docID int64) (*models.Document, error)
	GetDocumentsByUserID(ctx context.Context, userID int64, page, size int) ([]*models.Document, error)
	UpdateDocumentProgress(ctx context.Context, docID, userID int64, pageNumber int) error
	DeleteDocument(ctx context.Context, docID, userID int64) error
	DeleteScansByDocument(ctx context.Context, docID, userID int64) error
	GetScanByDocumentPage(ctx context.Context, documentID int64, pageNumber int) (*models.Scan, error)
	CreateScanFromDocument(ctx context.Context, scan *models.Scan) (int64, error)

	CreateAnnotation(ctx context.Context, annotation *models.Annotation) (int64, error)
	GetAnnotationByID(ctx context.Context, annotationID int64) (*models.Annotation, error)
	GetAnnotationsByUserID(ctx context.Context, userID int64, page, size int) ([]*models.Annotation, error)
	GetAnnotationsByUserIDAndScanID(ctx context.Context, userID, scanID int64, page, size int) ([]*models.Annotation, error)
	GetAnnotationsByUserIDAndDocumentPage(ctx context.Context, userID, documentID int64, pageNumber *int, page, size int) ([]*models.Annotation, error)
	DeleteAnnotation(ctx context.Context, annotationID, userID int64) error
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

func (s *postgresDB) DeleteUser(ctx context.Context, userID int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
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
	if scan.SourceType == "" {
		scan.SourceType = "image"
	}
	if scan.Status == "" {
		scan.Status = "processing"
	}
	query := `
		INSERT INTO scans (user_id, image_url, full_ocr_text, detected_language, document_id, page_number, source_type, status, failure_reason, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`
	err := s.db.QueryRowContext(ctx, query,
		scan.UserID,
		scan.ImageURL,
		scan.FullOCRText,
		scan.DetectedLanguage,
		scan.DocumentID,
		scan.PageNumber,
		scan.SourceType,
		scan.Status,
		scan.FailureReason,
		scan.CreatedAt,
	).Scan(&scan.ID)
	return scan.ID, err
}

func (s *postgresDB) GetScanByID(ctx context.Context, scanID int64) (*models.Scan, error) {
	query := `
		SELECT id, user_id, image_url, full_ocr_text, detected_language, document_id, page_number, source_type, status, failure_reason, created_at
		FROM scans
		WHERE id = $1
	`
	var scan models.Scan
	var fullOCRText, detectedLanguage, sourceType, status, failureReason sql.NullString
	var documentID sql.NullInt64
	var pageNumber sql.NullInt32
	var createdAt time.Time

	err := s.db.QueryRowContext(ctx, query, scanID).Scan(
		&scan.ID,
		&scan.UserID,
		&scan.ImageURL,
		&fullOCRText,
		&detectedLanguage,
		&documentID,
		&pageNumber,
		&sourceType,
		&status,
		&failureReason,
		&createdAt,
	)
	if err != nil {
		return nil, err
	}

	applyScanFields(&scan, fullOCRText, detectedLanguage, documentID, pageNumber, sourceType, status, failureReason, createdAt)

	return &scan, nil
}

func (s *postgresDB) GetScansByUserID(ctx context.Context, userID int64, page, size int) ([]*models.Scan, error) {
	offset := (page - 1) * size
	query := `
		SELECT id, user_id, image_url, full_ocr_text, detected_language, document_id, page_number, source_type, status, failure_reason, created_at
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
		var fullOCRText, detectedLanguage, sourceType, status, failureReason sql.NullString
		var documentID sql.NullInt64
		var pageNumber sql.NullInt32
		var createdAt time.Time

		err := rows.Scan(
			&scan.ID,
			&scan.UserID,
			&scan.ImageURL,
			&fullOCRText,
			&detectedLanguage,
			&documentID,
			&pageNumber,
			&sourceType,
			&status,
			&failureReason,
			&createdAt,
		)
		if err != nil {
			return nil, err
		}

		applyScanFields(&scan, fullOCRText, detectedLanguage, documentID, pageNumber, sourceType, status, failureReason, createdAt)

		scans = append(scans, &scan)
	}

	return scans, rows.Err()
}

func (s *postgresDB) UpdateScanOCR(ctx context.Context, scanID int64, text, language string) error {
	query := `
		UPDATE scans
		SET full_ocr_text = $1, detected_language = $2, status = 'ready', failure_reason = NULL
		WHERE id = $3
	`
	_, err := s.db.ExecContext(ctx, query, text, language, scanID)
	return err
}

func (s *postgresDB) UpdateScanStatus(ctx context.Context, scanID int64, status string, failureReason *string) error {
	query := `
		UPDATE scans
		SET status = $1, failure_reason = $2
		WHERE id = $3
	`
	_, err := s.db.ExecContext(ctx, query, status, failureReason, scanID)
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

func (s *postgresDB) DeleteScan(ctx context.Context, scanID, userID int64) error {
	query := `DELETE FROM scans WHERE id = $1 AND user_id = $2`
	result, err := s.db.ExecContext(ctx, query, scanID, userID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *postgresDB) CreateDocument(ctx context.Context, doc *models.Document) (int64, error) {
	if doc.LastPageNumber <= 0 {
		doc.LastPageNumber = 1
	}
	if doc.UpdatedAt.IsZero() {
		doc.UpdatedAt = doc.CreatedAt
	}
	query := `
		INSERT INTO documents (user_id, file_url, filename, page_count, file_size, last_page_number, last_opened_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`
	err := s.db.QueryRowContext(ctx, query,
		doc.UserID,
		doc.FileURL,
		doc.Filename,
		doc.PageCount,
		doc.FileSize,
		doc.LastPageNumber,
		doc.LastOpenedAt,
		doc.CreatedAt,
		doc.UpdatedAt,
	).Scan(&doc.ID)
	return doc.ID, err
}

func (s *postgresDB) UpdateDocumentFileInfo(ctx context.Context, docID int64, fileURL string, pageCount int) error {
	query := `
		UPDATE documents
		SET file_url = $1, page_count = $2, updated_at = $3
		WHERE id = $4
	`
	_, err := s.db.ExecContext(ctx, query, fileURL, pageCount, time.Now(), docID)
	return err
}

func (s *postgresDB) GetDocumentByID(ctx context.Context, docID int64) (*models.Document, error) {
	query := `
		SELECT id, user_id, file_url, filename, page_count, file_size, last_page_number, last_opened_at, created_at, updated_at
		FROM documents
		WHERE id = $1
	`
	var doc models.Document
	var lastOpenedAt sql.NullTime
	var createdAt, updatedAt time.Time

	err := s.db.QueryRowContext(ctx, query, docID).Scan(
		&doc.ID,
		&doc.UserID,
		&doc.FileURL,
		&doc.Filename,
		&doc.PageCount,
		&doc.FileSize,
		&doc.LastPageNumber,
		&lastOpenedAt,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if lastOpenedAt.Valid {
		doc.LastOpenedAt = &lastOpenedAt.Time
	}
	doc.CreatedAt = createdAt
	doc.UpdatedAt = updatedAt

	return &doc, nil
}

func (s *postgresDB) GetDocumentsByUserID(ctx context.Context, userID int64, page, size int) ([]*models.Document, error) {
	offset := (page - 1) * size
	query := `
		SELECT id, user_id, file_url, filename, page_count, file_size, last_page_number, last_opened_at, created_at, updated_at
		FROM documents
		WHERE user_id = $1
		ORDER BY COALESCE(last_opened_at, created_at) DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := s.db.QueryContext(ctx, query, userID, size, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []*models.Document
	for rows.Next() {
		var doc models.Document
		var lastOpenedAt sql.NullTime
		if err := rows.Scan(
			&doc.ID,
			&doc.UserID,
			&doc.FileURL,
			&doc.Filename,
			&doc.PageCount,
			&doc.FileSize,
			&doc.LastPageNumber,
			&lastOpenedAt,
			&doc.CreatedAt,
			&doc.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if lastOpenedAt.Valid {
			doc.LastOpenedAt = &lastOpenedAt.Time
		}
		docs = append(docs, &doc)
	}
	return docs, rows.Err()
}

func (s *postgresDB) UpdateDocumentProgress(ctx context.Context, docID, userID int64, pageNumber int) error {
	query := `
		UPDATE documents
		SET last_page_number = $1, last_opened_at = $2, updated_at = $2
		WHERE id = $3 AND user_id = $4
	`
	result, err := s.db.ExecContext(ctx, query, pageNumber, time.Now(), docID, userID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *postgresDB) DeleteScansByDocument(ctx context.Context, docID, userID int64) error {
	query := `DELETE FROM scans WHERE document_id = $1 AND user_id = $2`
	_, err := s.db.ExecContext(ctx, query, docID, userID)
	return err
}

func (s *postgresDB) DeleteDocument(ctx context.Context, docID, userID int64) error {
	query := `DELETE FROM documents WHERE id = $1 AND user_id = $2`
	result, err := s.db.ExecContext(ctx, query, docID, userID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *postgresDB) GetScanByDocumentPage(ctx context.Context, documentID int64, pageNumber int) (*models.Scan, error) {
	query := `
		SELECT id, user_id, image_url, full_ocr_text, detected_language, document_id, page_number, source_type, status, failure_reason, created_at
		FROM scans
		WHERE document_id = $1 AND page_number = $2
	`
	var scan models.Scan
	var fullOCRText, detectedLanguage, sourceType, status, failureReason sql.NullString
	var docID sql.NullInt64
	var pageNum sql.NullInt32
	var createdAt time.Time

	err := s.db.QueryRowContext(ctx, query, documentID, pageNumber).Scan(
		&scan.ID,
		&scan.UserID,
		&scan.ImageURL,
		&fullOCRText,
		&detectedLanguage,
		&docID,
		&pageNum,
		&sourceType,
		&status,
		&failureReason,
		&createdAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	applyScanFields(&scan, fullOCRText, detectedLanguage, docID, pageNum, sourceType, status, failureReason, createdAt)

	return &scan, nil
}

func (s *postgresDB) CreateScanFromDocument(ctx context.Context, scan *models.Scan) (int64, error) {
	if scan.SourceType == "" {
		scan.SourceType = "pdf"
	}
	if scan.Status == "" {
		scan.Status = "ready"
	}
	query := `
		INSERT INTO scans (user_id, image_url, full_ocr_text, detected_language, document_id, page_number, source_type, status, failure_reason, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`
	err := s.db.QueryRowContext(ctx, query,
		scan.UserID,
		scan.ImageURL,
		scan.FullOCRText,
		scan.DetectedLanguage,
		scan.DocumentID,
		scan.PageNumber,
		scan.SourceType,
		scan.Status,
		scan.FailureReason,
		scan.CreatedAt,
	).Scan(&scan.ID)
	return scan.ID, err
}

func (s *postgresDB) CreateAnnotation(ctx context.Context, annotation *models.Annotation) (int64, error) {
	nuanceJSON, err := json.Marshal(annotation.NuanceData)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal nuance_data: %w", err)
	}

	query := `
		INSERT INTO annotations (user_id, scan_id, source_type, document_id, page_number, source_label, highlighted_text, context_text, nuance_data, is_bookmarked, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id
	`
	err = s.db.QueryRowContext(ctx, query,
		annotation.UserID,
		annotation.ScanID,
		annotation.SourceType,
		annotation.DocumentID,
		annotation.PageNumber,
		annotation.SourceLabel,
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
		SELECT id, user_id, scan_id, source_type, document_id, page_number, source_label, highlighted_text, context_text, nuance_data, is_bookmarked, created_at
		FROM annotations
		WHERE id = $1
	`
	var annotation models.Annotation
	var scanID, documentID sql.NullInt64
	var pageNumber sql.NullInt32
	var sourceType, sourceLabel, contextText sql.NullString
	var nuanceData []byte
	var createdAt time.Time

	err := s.db.QueryRowContext(ctx, query, annotationID).Scan(
		&annotation.ID,
		&annotation.UserID,
		&scanID,
		&sourceType,
		&documentID,
		&pageNumber,
		&sourceLabel,
		&annotation.HighlightedText,
		&contextText,
		&nuanceData,
		&annotation.IsBookmarked,
		&createdAt,
	)
	if err != nil {
		return nil, err
	}

	if err := applyAnnotationFields(&annotation, scanID, sourceType, documentID, pageNumber, sourceLabel, contextText, nuanceData, createdAt); err != nil {
		return nil, fmt.Errorf("failed to unmarshal nuance_data: %w", err)
	}

	return &annotation, nil
}

func (s *postgresDB) GetAnnotationsByUserID(ctx context.Context, userID int64, page, size int) ([]*models.Annotation, error) {
	offset := (page - 1) * size
	query := `
		SELECT id, user_id, scan_id, source_type, document_id, page_number, source_label, highlighted_text, context_text, nuance_data, is_bookmarked, created_at
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

	return scanAnnotationRows(rows)
}

func (s *postgresDB) DeleteAnnotation(ctx context.Context, annotationID, userID int64) error {
	query := `DELETE FROM annotations WHERE id = $1 AND user_id = $2`
	result, err := s.db.ExecContext(ctx, query, annotationID, userID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *postgresDB) GetAnnotationsByUserIDAndScanID(
	ctx context.Context,
	userID, scanID int64,
	page, size int,
) ([]*models.Annotation, error) {
	offset := (page - 1) * size
	query := `
		SELECT id, user_id, scan_id, source_type, document_id, page_number, source_label, highlighted_text, context_text, nuance_data, is_bookmarked, created_at
		FROM annotations
		WHERE user_id = $1 AND scan_id = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`
	rows, err := s.db.QueryContext(ctx, query, userID, scanID, size, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanAnnotationRows(rows)
}

func (s *postgresDB) GetAnnotationsByUserIDAndDocumentPage(
	ctx context.Context,
	userID, documentID int64,
	pageNumber *int,
	page, size int,
) ([]*models.Annotation, error) {
	offset := (page - 1) * size
	args := []any{userID, documentID}
	pageClause := ""
	if pageNumber != nil {
		pageClause = " AND page_number = $3"
		args = append(args, *pageNumber)
	}
	args = append(args, size, offset)
	limitArg := len(args) - 1
	offsetArg := len(args)
	query := fmt.Sprintf(`
		SELECT id, user_id, scan_id, source_type, document_id, page_number, source_label, highlighted_text, context_text, nuance_data, is_bookmarked, created_at
		FROM annotations
		WHERE user_id = $1 AND document_id = $2%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, pageClause, limitArg, offsetArg)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAnnotationRows(rows)
}

func scanAnnotationRows(rows *sql.Rows) ([]*models.Annotation, error) {
	var annotations []*models.Annotation
	for rows.Next() {
		var annotation models.Annotation
		var scanID, documentID sql.NullInt64
		var pageNumber sql.NullInt32
		var sourceType, sourceLabel, contextText sql.NullString
		var nuanceData []byte
		var createdAt time.Time

		err := rows.Scan(
			&annotation.ID,
			&annotation.UserID,
			&scanID,
			&sourceType,
			&documentID,
			&pageNumber,
			&sourceLabel,
			&annotation.HighlightedText,
			&contextText,
			&nuanceData,
			&annotation.IsBookmarked,
			&createdAt,
		)
		if err != nil {
			return nil, err
		}

		if err := applyAnnotationFields(&annotation, scanID, sourceType, documentID, pageNumber, sourceLabel, contextText, nuanceData, createdAt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal nuance_data: %w", err)
		}

		annotations = append(annotations, &annotation)
	}
	return annotations, rows.Err()
}

func applyScanFields(
	scan *models.Scan,
	fullOCRText sql.NullString,
	detectedLanguage sql.NullString,
	documentID sql.NullInt64,
	pageNumber sql.NullInt32,
	sourceType sql.NullString,
	status sql.NullString,
	failureReason sql.NullString,
	createdAt time.Time,
) {
	if fullOCRText.Valid {
		scan.FullOCRText = &fullOCRText.String
	}
	if detectedLanguage.Valid {
		scan.DetectedLanguage = &detectedLanguage.String
	}
	if documentID.Valid {
		scan.DocumentID = &documentID.Int64
	}
	if pageNumber.Valid {
		pn := int(pageNumber.Int32)
		scan.PageNumber = &pn
	}
	if sourceType.Valid {
		scan.SourceType = sourceType.String
	}
	if status.Valid {
		scan.Status = status.String
	}
	if failureReason.Valid {
		scan.FailureReason = &failureReason.String
	}
	scan.CreatedAt = createdAt
}

func applyAnnotationFields(
	annotation *models.Annotation,
	scanID sql.NullInt64,
	sourceType sql.NullString,
	documentID sql.NullInt64,
	pageNumber sql.NullInt32,
	sourceLabel sql.NullString,
	contextText sql.NullString,
	nuanceData []byte,
	createdAt time.Time,
) error {
	if scanID.Valid {
		annotation.ScanID = &scanID.Int64
	}
	if sourceType.Valid {
		annotation.SourceType = &sourceType.String
	}
	if documentID.Valid {
		annotation.DocumentID = &documentID.Int64
	}
	if pageNumber.Valid {
		pn := int(pageNumber.Int32)
		annotation.PageNumber = &pn
	}
	if sourceLabel.Valid {
		annotation.SourceLabel = &sourceLabel.String
	}
	if contextText.Valid {
		annotation.ContextText = &contextText.String
	}
	if err := json.Unmarshal(nuanceData, &annotation.NuanceData); err != nil {
		return err
	}
	annotation.CreatedAt = createdAt
	return nil
}
