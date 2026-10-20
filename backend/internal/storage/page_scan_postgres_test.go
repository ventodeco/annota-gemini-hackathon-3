package storage

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gemini-hackathon/app/internal/models"
)

func TestGetOrCreateDocumentPageScanConcurrent_Postgres(t *testing.T) {
	store, pool := newMigratedPostgresStore(t)
	owner := createPGUser(t, store, "pagescan-owner")
	other := createPGUser(t, store, "pagescan-other")
	doc := createPGDocument(t, store, owner.ID, "concurrent.pdf", pgBaseTime)

	const creators = 8
	var wg sync.WaitGroup
	ids := make([]int64, creators)
	created := make([]bool, creators)
	errs := make([]error, creators)
	for i := range creators {
		wg.Go(func() {
			text := "ページ"
			page := 2
			ids[i], created[i], errs[i] = store.GetOrCreateDocumentPageScan(context.Background(), &models.Scan{
				UserID: owner.ID, FullOCRText: &text, DocumentID: &doc.ID, PageNumber: &page, CreatedAt: pgBaseTime,
			})
		})
	}
	wg.Wait()

	createdCount := 0
	for i := range creators {
		if errs[i] != nil {
			t.Fatalf("creator %d: %v", i, errs[i])
		}
		if ids[i] != ids[0] {
			t.Fatalf("creators got different scans: %v", ids)
		}
		if created[i] {
			createdCount++
		}
	}
	if createdCount != 1 {
		t.Fatalf("created = %d, want exactly one creator", createdCount)
	}
	var rows int
	if err := pool.QueryRow(`SELECT count(*) FROM scans WHERE document_id = $1 AND page_number = 2`, doc.ID).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("page scan rows = %d, want 1", rows)
	}

	text := "x"
	page := 1
	if _, _, err := store.GetOrCreateDocumentPageScan(context.Background(), &models.Scan{
		UserID: other.ID, FullOCRText: &text, DocumentID: &doc.ID, PageNumber: &page, CreatedAt: pgBaseTime,
	}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("non-owner create err = %v, want sql.ErrNoRows", err)
	}

	if err := store.DeleteDocumentAndPageScans(context.Background(), doc.ID, owner.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.GetOrCreateDocumentPageScan(context.Background(), &models.Scan{
		UserID: owner.ID, FullOCRText: &text, DocumentID: &doc.ID, PageNumber: &page, CreatedAt: pgBaseTime,
	}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("create after delete err = %v, want sql.ErrNoRows", err)
	}
}

func TestFailStaleProcessingScanIsConditional_Postgres(t *testing.T) {
	store, _ := newMigratedPostgresStore(t)
	user := createPGUser(t, store, "stale-scan")
	ctx := context.Background()
	old := time.Now().Add(-time.Hour)

	staleID, err := store.CreateScan(ctx, &models.Scan{UserID: user.ID, SourceType: "image", Status: "processing", CreatedAt: old})
	if err != nil {
		t.Fatal(err)
	}
	changed, err := store.FailStaleProcessingScan(ctx, staleID, time.Now().Add(-time.Minute), "ocr_interrupted")
	if err != nil || !changed {
		t.Fatalf("stale processing scan: changed=%v err=%v, want changed", changed, err)
	}
	got, _ := store.GetScanByID(ctx, staleID)
	if got.Status != "failed" || got.FailureReason == nil || *got.FailureReason != "ocr_interrupted" {
		t.Fatalf("stale scan = %q %v", got.Status, got.FailureReason)
	}

	readyID, _ := store.CreateScan(ctx, &models.Scan{UserID: user.ID, SourceType: "image", Status: "processing", CreatedAt: old})
	if err := store.UpdateScanOCR(ctx, readyID, "駅前", "ja", nil); err != nil {
		t.Fatal(err)
	}
	freshID, _ := store.CreateScan(ctx, &models.Scan{UserID: user.ID, SourceType: "image", Status: "processing", CreatedAt: time.Now()})
	for _, id := range []int64{readyID, freshID} {
		changed, err := store.FailStaleProcessingScan(ctx, id, time.Now().Add(-time.Minute), "ocr_interrupted")
		if err != nil || changed {
			t.Fatalf("scan %d: changed=%v err=%v, want unchanged", id, changed, err)
		}
	}
	if got, _ := store.GetScanByID(ctx, readyID); got.Status != "ready" {
		t.Fatalf("ready scan overwritten: %q", got.Status)
	}
}
