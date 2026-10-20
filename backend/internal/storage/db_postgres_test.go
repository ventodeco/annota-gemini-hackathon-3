package storage

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/gemini-hackathon/app/internal/models"
)

// newMigratedPostgresStore returns a store backed by a fresh schema with the
// real production migrations applied. The schema is dropped on cleanup.
func newMigratedPostgresStore(t *testing.T) (DB, *sql.DB) {
	t.Helper()
	pool := openPostgresPoolInSchema(t, postgresTestDSN(t))
	if err := RunMigrations(pool, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}
	return NewPostgresDB(pool), pool
}

var pgBaseTime = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func createPGUser(t *testing.T, store DB, suffix string) *models.User {
	t.Helper()
	user := &models.User{
		Email:             suffix + "@example.com",
		Provider:          "clerk",
		ProviderID:        "user_" + suffix,
		PreferredLanguage: "EN",
		CreatedAt:         pgBaseTime,
		UpdatedAt:         pgBaseTime,
	}
	if err := store.CreateUser(context.Background(), user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	return user
}

func createPGDocument(t *testing.T, store DB, userID int64, filename string, createdAt time.Time) *models.Document {
	t.Helper()
	doc := &models.Document{
		UserID:    userID,
		FileURL:   "/uploads/documents/" + filename,
		Filename:  filename,
		PageCount: 3,
		FileSize:  2048,
		CreatedAt: createdAt,
	}
	if _, err := store.CreateDocument(context.Background(), doc); err != nil {
		t.Fatalf("CreateDocument failed: %v", err)
	}
	return doc
}

func createPGPageScan(t *testing.T, store DB, userID, docID int64, page int) *models.Scan {
	t.Helper()
	text := "ページの本文"
	scan := &models.Scan{
		UserID:      userID,
		FullOCRText: &text,
		DocumentID:  &docID,
		PageNumber:  &page,
		CreatedAt:   pgBaseTime,
	}
	if _, err := store.CreateScanFromDocument(context.Background(), scan); err != nil {
		t.Fatalf("CreateScanFromDocument failed: %v", err)
	}
	return scan
}

func createPGDocumentAnnotation(t *testing.T, store DB, userID, docID int64, page int, text string, createdAt time.Time) int64 {
	t.Helper()
	sourceType := "pdf"
	annotation := &models.Annotation{
		UserID:          userID,
		SourceType:      &sourceType,
		DocumentID:      &docID,
		PageNumber:      &page,
		HighlightedText: text,
		NuanceData:      models.NuanceData{Meaning: text},
		IsBookmarked:    true,
		CreatedAt:       createdAt,
	}
	id, err := store.CreateAnnotation(context.Background(), annotation)
	if err != nil {
		t.Fatalf("CreateAnnotation failed: %v", err)
	}
	return id
}

func documentIDs(docs []*models.Document) []int64 {
	ids := make([]int64, 0, len(docs))
	for _, doc := range docs {
		ids = append(ids, doc.ID)
	}
	return ids
}

func annotationTexts(annotations []*models.Annotation) []string {
	texts := make([]string, 0, len(annotations))
	for _, annotation := range annotations {
		texts = append(texts, annotation.HighlightedText)
	}
	return texts
}

func TestDocumentLifecycle_Postgres(t *testing.T) {
	store, _ := newMigratedPostgresStore(t)
	ctx := context.Background()
	owner := createPGUser(t, store, "doc_owner")
	other := createPGUser(t, store, "doc_other")

	doc := createPGDocument(t, store, owner.ID, "book.pdf", pgBaseTime)
	if doc.ID == 0 || doc.LastPageNumber != 1 || !doc.UpdatedAt.Equal(pgBaseTime) {
		t.Fatalf("CreateDocument defaults = id %d, last page %d, updated %v", doc.ID, doc.LastPageNumber, doc.UpdatedAt)
	}

	got, err := store.GetDocumentByID(ctx, doc.ID)
	if err != nil || got == nil {
		t.Fatalf("GetDocumentByID = %v, %v", got, err)
	}
	if got.UserID != owner.ID || got.FileURL != doc.FileURL || got.Filename != "book.pdf" ||
		got.PageCount != 3 || got.FileSize != 2048 || got.LastPageNumber != 1 || got.LastOpenedAt != nil ||
		!got.CreatedAt.Equal(pgBaseTime) || !got.UpdatedAt.Equal(pgBaseTime) {
		t.Fatalf("GetDocumentByID round trip mismatch: %+v", got)
	}

	if missing, err := store.GetDocumentByID(ctx, doc.ID+1000); missing != nil || err != nil {
		t.Fatalf("GetDocumentByID(missing) = %v, %v; want nil, nil", missing, err)
	}

	if err := store.UpdateDocumentFileInfo(ctx, doc.ID, "/uploads/documents/renamed.pdf", 5); err != nil {
		t.Fatalf("UpdateDocumentFileInfo failed: %v", err)
	}
	got, _ = store.GetDocumentByID(ctx, doc.ID)
	if got.FileURL != "/uploads/documents/renamed.pdf" || got.PageCount != 5 || !got.UpdatedAt.After(pgBaseTime) {
		t.Fatalf("UpdateDocumentFileInfo not persisted: %+v", got)
	}

	if err := store.UpdateDocumentProgress(ctx, doc.ID, other.ID, 4); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("UpdateDocumentProgress(non-owner) error = %v, want sql.ErrNoRows", err)
	}
	got, _ = store.GetDocumentByID(ctx, doc.ID)
	if got.LastPageNumber != 1 || got.LastOpenedAt != nil {
		t.Fatalf("non-owner progress update changed document: %+v", got)
	}

	if err := store.UpdateDocumentProgress(ctx, doc.ID, owner.ID, 4); err != nil {
		t.Fatalf("UpdateDocumentProgress(owner) failed: %v", err)
	}
	got, _ = store.GetDocumentByID(ctx, doc.ID)
	if got.LastPageNumber != 4 || got.LastOpenedAt == nil || !got.LastOpenedAt.After(pgBaseTime) {
		t.Fatalf("owner progress update not persisted: %+v", got)
	}
}

func TestGetDocumentsByUserID_OrdersByRecentActivity_Postgres(t *testing.T) {
	store, _ := newMigratedPostgresStore(t)
	ctx := context.Background()
	owner := createPGUser(t, store, "list_owner")
	other := createPGUser(t, store, "list_other")

	oldest := createPGDocument(t, store, owner.ID, "oldest.pdf", pgBaseTime)
	middle := createPGDocument(t, store, owner.ID, "middle.pdf", pgBaseTime.Add(time.Hour))
	newest := createPGDocument(t, store, owner.ID, "newest.pdf", pgBaseTime.Add(2*time.Hour))
	createPGDocument(t, store, other.ID, "other.pdf", pgBaseTime.Add(3*time.Hour))

	// Opening the oldest document makes it the most recently active one.
	if err := store.UpdateDocumentProgress(ctx, oldest.ID, owner.ID, 2); err != nil {
		t.Fatalf("UpdateDocumentProgress failed: %v", err)
	}

	all, err := store.GetDocumentsByUserID(ctx, owner.ID, 1, 10)
	if err != nil {
		t.Fatalf("GetDocumentsByUserID failed: %v", err)
	}
	if want := []int64{oldest.ID, newest.ID, middle.ID}; !slices.Equal(documentIDs(all), want) {
		t.Fatalf("document order = %v, want %v", documentIDs(all), want)
	}
	if all[0].LastOpenedAt == nil || all[1].LastOpenedAt != nil {
		t.Fatalf("last_opened_at not mapped: %+v, %+v", all[0], all[1])
	}

	pageTwo, err := store.GetDocumentsByUserID(ctx, owner.ID, 2, 2)
	if err != nil {
		t.Fatalf("GetDocumentsByUserID(page 2) failed: %v", err)
	}
	if want := []int64{middle.ID}; !slices.Equal(documentIDs(pageTwo), want) {
		t.Fatalf("page 2 = %v, want %v", documentIDs(pageTwo), want)
	}
}

func TestDocumentPageScans_Postgres(t *testing.T) {
	store, _ := newMigratedPostgresStore(t)
	ctx := context.Background()
	owner := createPGUser(t, store, "page_owner")
	doc := createPGDocument(t, store, owner.ID, "pages.pdf", pgBaseTime)

	scan := createPGPageScan(t, store, owner.ID, doc.ID, 2)
	if scan.ID == 0 || scan.SourceType != "pdf" || scan.Status != "ready" {
		t.Fatalf("CreateScanFromDocument defaults = %+v", scan)
	}

	got, err := store.GetScanByDocumentPage(ctx, doc.ID, 2)
	if err != nil || got == nil {
		t.Fatalf("GetScanByDocumentPage = %v, %v", got, err)
	}
	if got.ID != scan.ID || got.UserID != owner.ID || got.DocumentID == nil || *got.DocumentID != doc.ID ||
		got.PageNumber == nil || *got.PageNumber != 2 || got.SourceType != "pdf" || got.Status != "ready" ||
		got.FullOCRText == nil || *got.FullOCRText != "ページの本文" {
		t.Fatalf("GetScanByDocumentPage round trip mismatch: %+v", got)
	}

	if missing, err := store.GetScanByDocumentPage(ctx, doc.ID, 3); missing != nil || err != nil {
		t.Fatalf("GetScanByDocumentPage(missing page) = %v, %v; want nil, nil", missing, err)
	}

	reason := "provider timeout"
	if err := store.UpdateScanStatus(ctx, scan.ID, "failed", &reason); err != nil {
		t.Fatalf("UpdateScanStatus failed: %v", err)
	}
	got, err = store.GetScanByID(ctx, scan.ID)
	if err != nil || got.Status != "failed" || got.FailureReason == nil || *got.FailureReason != reason {
		t.Fatalf("failed status not persisted: %+v, %v", got, err)
	}

	if err := store.UpdateScanOCR(ctx, scan.ID, "再処理された本文", "ja"); err != nil {
		t.Fatalf("UpdateScanOCR failed: %v", err)
	}
	got, err = store.GetScanByID(ctx, scan.ID)
	if err != nil || got.Status != "ready" || got.FailureReason != nil ||
		got.FullOCRText == nil || *got.FullOCRText != "再処理された本文" ||
		got.DetectedLanguage == nil || *got.DetectedLanguage != "ja" {
		t.Fatalf("OCR retry did not mark scan ready: %+v, %v", got, err)
	}
}

func TestDeleteDocument_Postgres(t *testing.T) {
	store, _ := newMigratedPostgresStore(t)
	ctx := context.Background()
	owner := createPGUser(t, store, "del_owner")
	other := createPGUser(t, store, "del_other")
	doc := createPGDocument(t, store, owner.ID, "delete.pdf", pgBaseTime)
	createPGPageScan(t, store, owner.ID, doc.ID, 1)
	createPGPageScan(t, store, owner.ID, doc.ID, 2)
	imageScanID, err := store.CreateScan(ctx, &models.Scan{UserID: owner.ID, ImageURL: "/uploads/photo.jpg", CreatedAt: pgBaseTime})
	if err != nil {
		t.Fatalf("CreateScan failed: %v", err)
	}

	if err := store.DeleteDocument(ctx, doc.ID, other.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("DeleteDocument(non-owner) error = %v, want sql.ErrNoRows", err)
	}
	if got, _ := store.GetDocumentByID(ctx, doc.ID); got == nil {
		t.Fatal("non-owner delete removed the document")
	}

	// Same order as the delete-document handler.
	if err := store.DeleteScansByDocument(ctx, doc.ID, owner.ID); err != nil {
		t.Fatalf("DeleteScansByDocument failed: %v", err)
	}
	if err := store.DeleteDocument(ctx, doc.ID, owner.ID); err != nil {
		t.Fatalf("DeleteDocument failed: %v", err)
	}

	if got, err := store.GetDocumentByID(ctx, doc.ID); got != nil || err != nil {
		t.Fatalf("GetDocumentByID after delete = %v, %v; want nil, nil", got, err)
	}
	for _, page := range []int{1, 2} {
		if got, err := store.GetScanByDocumentPage(ctx, doc.ID, page); got != nil || err != nil {
			t.Fatalf("page %d scan after delete = %v, %v; want nil, nil", page, got, err)
		}
	}
	if got, err := store.GetScanByID(ctx, imageScanID); err != nil || got == nil {
		t.Fatalf("unrelated image scan removed by document delete: %v, %v", got, err)
	}
	if err := store.DeleteDocument(ctx, doc.ID, owner.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("repeat DeleteDocument error = %v, want sql.ErrNoRows", err)
	}
}

func TestAnnotationNuanceDataJSONB_Postgres(t *testing.T) {
	store, _ := newMigratedPostgresStore(t)
	ctx := context.Background()
	owner := createPGUser(t, store, "jsonb_owner")
	scanID, err := store.CreateScan(ctx, &models.Scan{UserID: owner.ID, ImageURL: "/uploads/page.jpg", CreatedAt: pgBaseTime})
	if err != nil {
		t.Fatalf("CreateScan failed: %v", err)
	}

	contextText := "彼は「大丈夫」と言った。"
	nuance := models.NuanceData{
		Translation:           "It's okay",
		ContextualExplanation: "Reassurance with \"quotes\" and a\nnewline",
		UsageExample:          "大丈夫ですか？",
		WordBreakdown:         "大 + 丈夫",
		Pronunciation:         models.PronunciationData{Kana: "だいじょうぶ", Romaji: "daijoubu"},
		Meaning:               "fine; all right",
	}
	id, err := store.CreateAnnotation(ctx, &models.Annotation{
		UserID:          owner.ID,
		ScanID:          &scanID,
		HighlightedText: "大丈夫",
		ContextText:     &contextText,
		NuanceData:      nuance,
		IsBookmarked:    false,
		CreatedAt:       pgBaseTime,
	})
	if err != nil {
		t.Fatalf("CreateAnnotation failed: %v", err)
	}

	got, err := store.GetAnnotationByID(ctx, id)
	if err != nil {
		t.Fatalf("GetAnnotationByID failed: %v", err)
	}
	if got.NuanceData != nuance {
		t.Fatalf("nuance_data round trip = %+v, want %+v", got.NuanceData, nuance)
	}
	if got.ScanID == nil || *got.ScanID != scanID || got.ContextText == nil || *got.ContextText != contextText ||
		got.IsBookmarked || !got.CreatedAt.Equal(pgBaseTime) {
		t.Fatalf("annotation round trip mismatch: %+v", got)
	}
}

func TestGetAnnotationsByUserIDAndDocumentPage_Postgres(t *testing.T) {
	store, _ := newMigratedPostgresStore(t)
	ctx := context.Background()
	owner := createPGUser(t, store, "ann_owner")
	other := createPGUser(t, store, "ann_other")
	doc := createPGDocument(t, store, owner.ID, "annotated.pdf", pgBaseTime)
	otherDoc := createPGDocument(t, store, owner.ID, "second.pdf", pgBaseTime)

	createPGDocumentAnnotation(t, store, owner.ID, doc.ID, 1, "p1-old", pgBaseTime)
	createPGDocumentAnnotation(t, store, owner.ID, doc.ID, 2, "p2", pgBaseTime.Add(time.Minute))
	createPGDocumentAnnotation(t, store, owner.ID, doc.ID, 1, "p1-new", pgBaseTime.Add(2*time.Minute))
	createPGDocumentAnnotation(t, store, owner.ID, otherDoc.ID, 1, "other-doc", pgBaseTime.Add(3*time.Minute))
	createPGDocumentAnnotation(t, store, other.ID, doc.ID, 1, "other-user", pgBaseTime.Add(4*time.Minute))

	page1 := 1
	tests := []struct {
		name       string
		userID     int64
		pageNumber *int
		page, size int
		want       []string
	}{
		{"all pages newest first", owner.ID, nil, 1, 10, []string{"p1-new", "p2", "p1-old"}},
		{"all pages second page", owner.ID, nil, 2, 2, []string{"p1-old"}},
		{"single page filter", owner.ID, &page1, 1, 10, []string{"p1-new", "p1-old"}},
		{"single page filter paginated", owner.ID, &page1, 2, 1, []string{"p1-old"}},
		{"other user's rows stay separate", other.ID, nil, 1, 10, []string{"other-user"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := store.GetAnnotationsByUserIDAndDocumentPage(ctx, tt.userID, doc.ID, tt.pageNumber, tt.page, tt.size)
			if err != nil {
				t.Fatalf("GetAnnotationsByUserIDAndDocumentPage failed: %v", err)
			}
			if texts := annotationTexts(got); !slices.Equal(texts, tt.want) {
				t.Fatalf("annotations = %v, want %v", texts, tt.want)
			}
			for _, annotation := range got {
				if annotation.DocumentID == nil || *annotation.DocumentID != doc.ID ||
					annotation.SourceType == nil || *annotation.SourceType != "pdf" {
					t.Fatalf("document source fields not mapped: %+v", annotation)
				}
			}
		})
	}
}
