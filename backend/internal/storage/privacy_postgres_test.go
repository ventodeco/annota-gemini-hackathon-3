package storage

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/gemini-hackathon/app/internal/models"
)

type privacyAccountRows struct {
	userID       int64
	imageScanID  int64
	documentID   int64
	pageScanID   int64
	annotationID int64
}

// TestAccountDeletionIsolationAndRetention_Postgres pins the stored-data
// lifecycle for #41 on the real schema. Rows are checked by the IDs captured
// at insert time, not by parent queries, so orphaned private rows can't pass.
func TestAccountDeletionIsolationAndRetention_Postgres(t *testing.T) {
	ctx := context.Background()
	store, pool := newMigratedPostgresStore(t)

	seed := func(t *testing.T, clerkUserID string) privacyAccountRows {
		t.Helper()
		user, err := store.ProvisionClerkUser(ctx, ClerkProvisionParams{ClerkUserID: clerkUserID, Email: clerkUserID + "@example.com"})
		if err != nil {
			t.Fatalf("ProvisionClerkUser: %v", err)
		}
		rows := privacyAccountRows{userID: user.ID}
		now := time.Now()
		text, page := "private text", 1
		if rows.imageScanID, err = store.CreateScan(ctx, &models.Scan{UserID: user.ID, FullOCRText: &text, CreatedAt: now}); err != nil {
			t.Fatalf("CreateScan: %v", err)
		}
		if rows.documentID, err = store.CreateDocument(ctx, &models.Document{UserID: user.ID, Filename: "book.pdf", CreatedAt: now}); err != nil {
			t.Fatalf("CreateDocument: %v", err)
		}
		if rows.pageScanID, err = store.CreateScanFromDocument(ctx, &models.Scan{
			UserID: user.ID, FullOCRText: &text, DocumentID: &rows.documentID, PageNumber: &page, CreatedAt: now,
		}); err != nil {
			t.Fatalf("CreateScanFromDocument: %v", err)
		}
		if rows.annotationID, err = store.CreateAnnotation(ctx, &models.Annotation{
			UserID: user.ID, ScanID: &rows.pageScanID, DocumentID: &rows.documentID, PageNumber: &page,
			HighlightedText: "saved highlight", NuanceData: models.NuanceData{Meaning: "meaning"}, CreatedAt: now,
		}); err != nil {
			t.Fatalf("CreateAnnotation: %v", err)
		}
		return rows
	}

	countByID := func(t *testing.T, table string, id int64) int {
		t.Helper()
		var n int
		if err := pool.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE id = $1`, id).Scan(&n); err != nil {
			t.Fatalf("count %s %d: %v", table, id, err)
		}
		return n
	}

	deleting := seed(t, "user_privacy_a")
	other := seed(t, "user_privacy_b")

	t.Run("document deletion removes page scans and keeps the saved annotation", func(t *testing.T) {
		// The same call DELETE /v1/documents/{id} makes after removing the PDF.
		if err := store.DeleteDocumentAndPageScans(ctx, deleting.documentID, deleting.userID); err != nil {
			t.Fatalf("DeleteDocumentAndPageScans: %v", err)
		}
		if n := countByID(t, "scans", deleting.pageScanID); n != 0 {
			t.Fatalf("page scan %d with extracted text still stored", deleting.pageScanID)
		}
		if n := countByID(t, "documents", deleting.documentID); n != 0 {
			t.Fatalf("document %d still stored", deleting.documentID)
		}

		annotation, err := store.GetAnnotationByID(ctx, deleting.annotationID)
		if err != nil || annotation == nil {
			t.Fatalf("annotation %d should be retained after source deletion: %v", deleting.annotationID, err)
		}
		if annotation.ScanID != nil {
			t.Fatalf("deleted page scan must be unlinked, got scan_id %d", *annotation.ScanID)
		}
		if annotation.HighlightedText != "saved highlight" {
			t.Fatalf("retained annotation snapshot changed: %q", annotation.HighlightedText)
		}
		// document_id has no FK: it stays as a historical reference and,
		// because ids are never reused, cannot point at another account's row.
		if annotation.DocumentID == nil || *annotation.DocumentID != deleting.documentID {
			t.Fatalf("document reference = %v, want historical %d", annotation.DocumentID, deleting.documentID)
		}
	})

	t.Run("a non-owner cannot delete another account's rows", func(t *testing.T) {
		if err := store.DeleteScan(ctx, other.imageScanID, deleting.userID); err == nil {
			t.Fatal("DeleteScan by a non-owner succeeded")
		}
		if err := store.DeleteDocumentAndPageScans(ctx, other.documentID, deleting.userID); err == nil {
			t.Fatal("DeleteDocumentAndPageScans by a non-owner succeeded")
		}
		if err := store.DeleteAnnotation(ctx, other.annotationID, deleting.userID); err == nil {
			t.Fatal("DeleteAnnotation by a non-owner succeeded")
		}
	})

	t.Run("account deletion removes every row of that account only", func(t *testing.T) {
		if err := store.DeleteUserAndTombstoneClerk(ctx, deleting.userID, "user_privacy_a"); err != nil {
			t.Fatalf("DeleteUserAndTombstoneClerk: %v", err)
		}

		for table, id := range map[string]int64{
			"users":       deleting.userID,
			"scans":       deleting.imageScanID,
			"annotations": deleting.annotationID,
		} {
			if n := countByID(t, table, id); n != 0 {
				t.Fatalf("%s row %d of the deleted account still stored", table, id)
			}
		}
		for _, table := range []string{"scans", "documents", "annotations"} {
			var n int
			if err := pool.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE user_id = $1`, deleting.userID).Scan(&n); err != nil {
				t.Fatalf("count %s: %v", table, err)
			}
			if n != 0 {
				t.Fatalf("%d %s rows of the deleted account remain", n, table)
			}
		}

		deleted, err := store.IsClerkUserDeleted(ctx, "user_privacy_a")
		if err != nil || !deleted {
			t.Fatalf("tombstone missing: deleted=%v err=%v", deleted, err)
		}

		for table, id := range map[string]int64{
			"users":       other.userID,
			"scans":       other.imageScanID,
			"documents":   other.documentID,
			"annotations": other.annotationID,
		} {
			if n := countByID(t, table, id); n != 1 {
				t.Fatalf("other account's %s row %d was removed", table, id)
			}
		}
		if n := countByID(t, "scans", other.pageScanID); n != 1 {
			t.Fatalf("other account's page scan %d was removed", other.pageScanID)
		}
		if err := store.DeleteUserAndTombstoneClerk(ctx, deleting.userID, "user_privacy_a"); err != sql.ErrNoRows {
			t.Fatalf("repeat local deletion = %v, want sql.ErrNoRows", err)
		}
	})
}

// waitForLockWaiter blocks until some session in this database waits on a lock.
func waitForLockWaiter(t *testing.T, pool *sql.DB) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var waiting int
		if err := pool.QueryRow(`SELECT COUNT(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(&waiting); err != nil {
			t.Fatalf("pg_stat_activity: %v", err)
		}
		if waiting > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("no session started waiting on a lock")
}

func TestDeleteDocumentAndPageScans_ConcurrentPageScan_Postgres(t *testing.T) {
	ctx := context.Background()
	store, pool := newMigratedPostgresStore(t)
	user, err := store.ProvisionClerkUser(ctx, ClerkProvisionParams{ClerkUserID: "user_doc_race", Email: "race@example.com"})
	if err != nil {
		t.Fatalf("ProvisionClerkUser: %v", err)
	}
	docID, err := store.CreateDocument(ctx, &models.Document{UserID: user.ID, Filename: "book.pdf", CreatedAt: time.Now()})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	// A page-scan request is mid-insert (uncommitted) when the document is deleted.
	insert, err := pool.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	defer insert.Rollback()
	var pageScanID int64
	if err := insert.QueryRowContext(ctx, `INSERT INTO scans (user_id, document_id, page_number, full_ocr_text, source_type, status)
		VALUES ($1, $2, 1, 'private page text', 'pdf', 'ready') RETURNING id`, user.ID, docID).Scan(&pageScanID); err != nil {
		t.Fatalf("insert page scan: %v", err)
	}

	deleted := make(chan error, 1)
	go func() { deleted <- store.DeleteDocumentAndPageScans(ctx, docID, user.ID) }()
	waitForLockWaiter(t, pool)
	if err := insert.Commit(); err != nil {
		t.Fatalf("commit page scan: %v", err)
	}
	if err := <-deleted; err != nil {
		t.Fatalf("DeleteDocumentAndPageScans: %v", err)
	}

	var remaining int
	if err := pool.QueryRowContext(ctx, `SELECT COUNT(*) FROM scans WHERE id = $1`, pageScanID).Scan(&remaining); err != nil {
		t.Fatalf("count scan: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("page scan %d with extracted text outlived its document", pageScanID)
	}
	if err := store.DeleteDocumentAndPageScans(ctx, docID, user.ID); err != sql.ErrNoRows {
		t.Fatalf("second delete = %v, want sql.ErrNoRows", err)
	}
}

func TestWithAccountLock_ExclusiveWaitsForShared_Postgres(t *testing.T) {
	ctx := context.Background()
	store, _ := newMigratedPostgresStore(t)

	inUpload := make(chan struct{})
	finishUpload := make(chan struct{})
	uploadDone := make(chan error, 1)
	go func() {
		uploadDone <- store.WithAccountLock(ctx, 42, false, func(context.Context) error {
			close(inUpload)
			<-finishUpload
			return nil
		})
	}()
	<-inUpload

	// Another shared holder (a second upload) is not blocked.
	if err := store.WithAccountLock(ctx, 42, false, func(context.Context) error { return nil }); err != nil {
		t.Fatalf("second shared lock: %v", err)
	}

	deletionDone := make(chan error, 1)
	go func() {
		deletionDone <- store.WithAccountLock(ctx, 42, true, func(context.Context) error { return nil })
	}()
	select {
	case err := <-deletionDone:
		t.Fatalf("exclusive lock acquired while an upload held the account (err=%v)", err)
	case <-time.After(150 * time.Millisecond):
	}
	// A different account is unaffected.
	if err := store.WithAccountLock(ctx, 43, true, func(context.Context) error { return nil }); err != nil {
		t.Fatalf("other account lock: %v", err)
	}

	close(finishUpload)
	if err := <-uploadDone; err != nil {
		t.Fatalf("upload lock: %v", err)
	}
	if err := <-deletionDone; err != nil {
		t.Fatalf("deletion lock: %v", err)
	}
}

func TestWithAccountLock_HeldAfterRequestCancellation_Postgres(t *testing.T) {
	store, _ := newMigratedPostgresStore(t)
	requestCtx, cancelRequest := context.WithCancel(context.Background())

	inUpload := make(chan struct{})
	finishUpload := make(chan struct{})
	uploadDone := make(chan error, 1)
	go func() {
		uploadDone <- store.WithAccountLock(requestCtx, 7, false, func(context.Context) error {
			close(inUpload)
			<-finishUpload // e.g. a file write that ignores cancellation
			return nil
		})
	}()
	<-inUpload
	cancelRequest() // the client disconnects mid-upload

	deletionDone := make(chan error, 1)
	go func() {
		deletionDone <- store.WithAccountLock(context.Background(), 7, true, func(context.Context) error { return nil })
	}()
	select {
	case err := <-deletionDone:
		t.Fatalf("account deletion acquired the lock while the cancelled upload was still running (err=%v)", err)
	case <-time.After(200 * time.Millisecond):
	}

	close(finishUpload)
	<-uploadDone
	if err := <-deletionDone; err != nil {
		t.Fatalf("deletion lock: %v", err)
	}
}

func TestWithAccountLock_WaitHonorsCancellation_Postgres(t *testing.T) {
	store, _ := newMigratedPostgresStore(t)
	release := make(chan struct{})
	holding := make(chan struct{})
	holderDone := make(chan error, 1)
	go func() {
		holderDone <- store.WithAccountLock(context.Background(), 8, true, func(context.Context) error {
			close(holding)
			<-release
			return nil
		})
	}()
	<-holding

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	ran := false
	err := store.WithAccountLock(ctx, 8, false, func(context.Context) error { ran = true; return nil })
	if err == nil || ran {
		t.Fatalf("waiting for the lock should stop on cancellation: err=%v ran=%v", err, ran)
	}

	close(release)
	if err := <-holderDone; err != nil {
		t.Fatalf("holder: %v", err)
	}
}

func TestGetUserUploads_ListsEveryRowOfOneUser_Postgres(t *testing.T) {
	ctx := context.Background()
	store, _ := newMigratedPostgresStore(t)
	owner, err := store.ProvisionClerkUser(ctx, ClerkProvisionParams{ClerkUserID: "user_uploads_a", Email: "ua@example.com"})
	if err != nil {
		t.Fatalf("ProvisionClerkUser: %v", err)
	}
	other, err := store.ProvisionClerkUser(ctx, ClerkProvisionParams{ClerkUserID: "user_uploads_b", Email: "ub@example.com"})
	if err != nil {
		t.Fatalf("ProvisionClerkUser: %v", err)
	}

	// More than one page of the paginated listings, which account deletion
	// no longer uses because concurrent progress updates reorder them.
	const n = 120
	wantDocs := map[int64]bool{}
	wantScans := map[int64]bool{}
	for i := 0; i < n; i++ {
		docID, err := store.CreateDocument(ctx, &models.Document{UserID: owner.ID, Filename: "b.pdf", CreatedAt: time.Now()})
		if err != nil {
			t.Fatalf("CreateDocument: %v", err)
		}
		wantDocs[docID] = true
		scanID, err := store.CreateScan(ctx, &models.Scan{UserID: owner.ID, CreatedAt: time.Now()})
		if err != nil {
			t.Fatalf("CreateScan: %v", err)
		}
		wantScans[scanID] = true
	}
	if _, err := store.CreateDocument(ctx, &models.Document{UserID: other.ID, Filename: "other.pdf", CreatedAt: time.Now()}); err != nil {
		t.Fatalf("CreateDocument other: %v", err)
	}

	scans, docs, err := store.GetUserUploads(ctx, owner.ID)
	if err != nil {
		t.Fatalf("GetUserUploads: %v", err)
	}
	if len(scans) != n || len(docs) != n {
		t.Fatalf("got %d scans and %d documents, want %d each", len(scans), len(docs), n)
	}
	for _, doc := range docs {
		if !wantDocs[doc.ID] || doc.UserID != owner.ID {
			t.Fatalf("unexpected document %+v", doc)
		}
	}
	for _, scan := range scans {
		if !wantScans[scan.ID] || scan.UserID != owner.ID {
			t.Fatalf("unexpected scan %+v", scan)
		}
	}
}
