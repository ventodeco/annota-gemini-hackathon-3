package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// openPostgresPoolInSchema opens a multi-connection pool whose sessions all
// use a fresh schema, so concurrent provisioning can be exercised.
func openPostgresPoolInSchema(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	_, schema := createPostgresMigrationSchema(t, dsn)

	var scoped string
	switch {
	case strings.Contains(dsn, "://") && strings.Contains(dsn, "?"):
		scoped = dsn + "&search_path=" + schema
	case strings.Contains(dsn, "://"):
		scoped = dsn + "?search_path=" + schema
	default:
		scoped = dsn + " search_path=" + schema
	}
	pool, err := sql.Open("postgres", scoped)
	if err != nil {
		t.Fatalf("open scoped pool: %v", err)
	}
	pool.SetMaxOpenConns(10)
	t.Cleanup(func() { pool.Close() })
	return pool
}

func TestClerkProvisioning_Postgres(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("set POSTGRES_TEST_DSN to run Clerk provisioning coverage against PostgreSQL")
	}
	ctx := context.Background()
	migrationsDir := filepath.Join("..", "..", "migrations")

	// Upgrade path: a pre-Clerk database with a legacy Google user and content.
	legacyDir := t.TempDir()
	for _, name := range []string{
		"001_initial_database_schema.sql",
		"002_add_documents.sql",
		"003_prd_reader_memory.sql",
		"004_add_annotations_scan_id_index.sql",
	} {
		content, err := os.ReadFile(filepath.Join(migrationsDir, name))
		if err != nil {
			t.Fatalf("read migration %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(legacyDir, name), content, 0600); err != nil {
			t.Fatalf("copy migration %s: %v", name, err)
		}
	}

	pool := openPostgresPoolInSchema(t, dsn)
	if err := RunMigrations(pool, legacyDir); err != nil {
		t.Fatalf("legacy migrations: %v", err)
	}
	var legacyID, otherLegacyID, scanID int64
	if err := pool.QueryRow(`INSERT INTO users (email, provider, provider_id, preferred_language)
		VALUES ('legacy@gmail.com', 'google', 'google-sub-1', 'JP') RETURNING id`).Scan(&legacyID); err != nil {
		t.Fatalf("seed legacy user: %v", err)
	}
	if err := pool.QueryRow(`INSERT INTO users (email, provider, provider_id)
		VALUES ('Second@Gmail.com', 'google', 'google-sub-2') RETURNING id`).Scan(&otherLegacyID); err != nil {
		t.Fatalf("seed second legacy user: %v", err)
	}
	if err := pool.QueryRow(`INSERT INTO scans (user_id, image_url) VALUES ($1, '/legacy.jpg') RETURNING id`, legacyID).Scan(&scanID); err != nil {
		t.Fatalf("seed legacy scan: %v", err)
	}

	if err := RunMigrations(pool, migrationsDir); err != nil {
		t.Fatalf("upgrade to Clerk migration: %v", err)
	}
	if err := RunMigrations(pool, migrationsDir); err != nil {
		t.Fatalf("re-running migrations: %v", err)
	}
	store := NewPostgresDB(pool)

	provisionConcurrently := func(t *testing.T, p ClerkProvisionParams) []int64 {
		t.Helper()
		const workers = 8
		ids := make([]int64, workers)
		errs := make([]error, workers)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := range workers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				user, err := store.ProvisionClerkUser(ctx, p)
				errs[i] = err
				if user != nil {
					ids[i] = user.ID
				}
			}()
		}
		close(start)
		wg.Wait()
		for i, err := range errs {
			if err != nil {
				t.Fatalf("worker %d: %v", i, err)
			}
		}
		return ids
	}
	countRows := func(t *testing.T, query string, args ...any) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(query, args...).Scan(&n); err != nil {
			t.Fatalf("count: %v", err)
		}
		return n
	}

	t.Run("concurrent first requests create exactly one profile", func(t *testing.T) {
		ids := provisionConcurrently(t, ClerkProvisionParams{ClerkUserID: "user_new", Email: "new@example.com"})
		for _, id := range ids {
			if id != ids[0] {
				t.Fatalf("workers returned different users: %v", ids)
			}
		}
		if n := countRows(t, `SELECT COUNT(*) FROM users WHERE clerk_user_id = 'user_new'`); n != 1 {
			t.Fatalf("expected one row, got %d", n)
		}
		user, err := store.GetUserByClerkID(ctx, "user_new")
		if err != nil || user == nil || user.Provider != "clerk" || user.ProviderID != "user_new" || user.Email != "new@example.com" {
			t.Fatalf("unexpected provisioned user %+v err=%v", user, err)
		}
	})

	t.Run("legacy google profile is linked with its numeric id and content", func(t *testing.T) {
		ids := provisionConcurrently(t, ClerkProvisionParams{
			ClerkUserID: "user_legacy", Email: "legacy@gmail.com", GoogleSubject: "google-sub-1",
		})
		for _, id := range ids {
			if id != legacyID {
				t.Fatalf("expected legacy id %d, got %v", legacyID, ids)
			}
		}
		user, err := store.GetUserByClerkID(ctx, "user_legacy")
		if err != nil || user == nil || user.ID != legacyID || user.PreferredLanguage != "JP" {
			t.Fatalf("legacy link lost data: %+v err=%v", user, err)
		}
		if n := countRows(t, `SELECT COUNT(*) FROM scans WHERE id = $1 AND user_id = $2`, scanID, legacyID); n != 1 {
			t.Fatalf("legacy scan ownership changed")
		}
	})

	t.Run("email-only sign-in cannot take over an unlinked legacy profile", func(t *testing.T) {
		before := countRows(t, `SELECT COUNT(*) FROM users`)
		_, err := store.ProvisionClerkUser(ctx, ClerkProvisionParams{ClerkUserID: "user_email_first", Email: "second@gmail.com"})
		if !errors.Is(err, ErrLegacyLinkRequired) {
			t.Fatalf("expected ErrLegacyLinkRequired, got %v", err)
		}
		if after := countRows(t, `SELECT COUNT(*) FROM users`); after != before {
			t.Fatalf("collision must not create a row: %d -> %d", before, after)
		}
		if n := countRows(t, `SELECT COUNT(*) FROM users WHERE id = $1 AND clerk_user_id IS NULL`, otherLegacyID); n != 1 {
			t.Fatalf("legacy row must stay unlinked")
		}
	})

	t.Run("deleted subject is tombstoned and never re-provisioned", func(t *testing.T) {
		user, err := store.GetUserByClerkID(ctx, "user_legacy")
		if err != nil || user == nil {
			t.Fatalf("lookup: %v", err)
		}
		if err := store.DeleteUserAndTombstoneClerk(ctx, user.ID, "user_legacy"); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if deleted, err := store.IsClerkUserDeleted(ctx, "user_legacy"); err != nil || !deleted {
			t.Fatalf("expected tombstone, got %v err=%v", deleted, err)
		}
		if n := countRows(t, `SELECT COUNT(*) FROM scans WHERE user_id = $1`, legacyID); n != 0 {
			t.Fatalf("owned scans must cascade on deletion")
		}
		if _, err := store.ProvisionClerkUser(ctx, ClerkProvisionParams{ClerkUserID: "user_legacy", Email: "legacy@gmail.com"}); !errors.Is(err, ErrClerkUserDeleted) {
			t.Fatalf("expected ErrClerkUserDeleted, got %v", err)
		}
		if err := store.DeleteUserAndTombstoneClerk(ctx, user.ID, "user_legacy"); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("second delete should report no rows, got %v", err)
		}
	})

	t.Run("clerk subject uniqueness is enforced by the schema", func(t *testing.T) {
		_, err := pool.Exec(`INSERT INTO users (email, provider, provider_id, clerk_user_id)
			VALUES ('dup@example.com', 'clerk', $1, 'user_new')`, fmt.Sprintf("dup-%d", time.Now().UnixNano()))
		if err == nil {
			t.Fatal("expected unique violation on clerk_user_id")
		}
	})
}
