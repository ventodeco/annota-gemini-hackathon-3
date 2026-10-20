package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/gemini-hackathon/app/internal/models"
)

var (
	// ErrClerkUserDeleted means the Clerk subject belongs to a deleted
	// ANNOTA account and must not be provisioned again.
	ErrClerkUserDeleted = errors.New("clerk user was deleted")
	// ErrLegacyLinkRequired means an unlinked legacy Google profile shares
	// the verified email. Ownership must be proven by Google sign-in rather
	// than transferred by email.
	ErrLegacyLinkRequired = errors.New("legacy_google_link_required")
)

// ClerkProvisionParams carries Clerk-verified identity facts for first-time
// provisioning.
type ClerkProvisionParams struct {
	ClerkUserID   string
	Email         string
	AvatarURL     string
	GoogleSubject string
}

const userColumns = `id, email, provider, provider_id, avatar_url, preferred_language, created_at, updated_at, clerk_user_id`

func (s *postgresDB) GetUserByClerkID(ctx context.Context, clerkUserID string) (*models.User, error) {
	user, err := s.scanUser(s.db.QueryRowContext(ctx,
		`SELECT `+userColumns+` FROM users WHERE clerk_user_id = $1`, clerkUserID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return user, err
}

func (s *postgresDB) IsClerkUserDeleted(ctx context.Context, clerkUserID string) (bool, error) {
	var deleted bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM deleted_clerk_users WHERE clerk_user_id = $1)`, clerkUserID).Scan(&deleted)
	return deleted, err
}

// ProvisionClerkUser returns the local user for a Clerk subject, linking a
// legacy Google profile or creating a new one. A per-subject advisory lock
// serializes concurrent first requests so exactly one profile results.
func (s *postgresDB) ProvisionClerkUser(ctx context.Context, p ClerkProvisionParams) (*models.User, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('clerk:' || $1))`, p.ClerkUserID); err != nil {
		return nil, fmt.Errorf("lock clerk subject: %w", err)
	}

	user, err := s.scanUser(tx.QueryRowContext(ctx,
		`SELECT `+userColumns+` FROM users WHERE clerk_user_id = $1`, p.ClerkUserID))
	if err == nil {
		return user, tx.Commit()
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	var deleted bool
	if err := tx.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM deleted_clerk_users WHERE clerk_user_id = $1)`, p.ClerkUserID).Scan(&deleted); err != nil {
		return nil, err
	}
	if deleted {
		return nil, ErrClerkUserDeleted
	}

	now := time.Now()
	if p.GoogleSubject != "" {
		user, err = s.scanUser(tx.QueryRowContext(ctx, `
			UPDATE users SET clerk_user_id = $1, updated_at = $2
			WHERE provider = 'google' AND provider_id = $3 AND clerk_user_id IS NULL
			RETURNING `+userColumns, p.ClerkUserID, now, p.GoogleSubject))
		if err == nil {
			return user, tx.Commit()
		}
		if err != sql.ErrNoRows {
			return nil, fmt.Errorf("link legacy google user: %w", err)
		}
	}

	var legacyEmailMatch bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM users
			WHERE provider = 'google' AND clerk_user_id IS NULL AND lower(email) = lower($1)
		)`, p.Email).Scan(&legacyEmailMatch); err != nil {
		return nil, err
	}
	if legacyEmailMatch {
		return nil, ErrLegacyLinkRequired
	}

	var avatarURL *string
	if p.AvatarURL != "" {
		avatarURL = &p.AvatarURL
	}
	user, err = s.scanUser(tx.QueryRowContext(ctx, `
		INSERT INTO users (email, provider, provider_id, avatar_url, preferred_language, created_at, updated_at, clerk_user_id)
		VALUES ($1, 'clerk', $2, $3, 'EN', $4, $4, $5)
		RETURNING `+userColumns, p.Email, p.ClerkUserID, avatarURL, now, p.ClerkUserID))
	if err != nil {
		return nil, fmt.Errorf("create clerk user: %w", err)
	}
	return user, tx.Commit()
}

// DeleteUserAndTombstoneClerk deletes the local profile (cascading owned rows)
// and permanently records the Clerk subject as deleted in one transaction.
func (s *postgresDB) DeleteUserAndTombstoneClerk(ctx context.Context, userID int64, clerkUserID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if clerkUserID != "" {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO deleted_clerk_users (clerk_user_id) VALUES ($1) ON CONFLICT (clerk_user_id) DO NOTHING`,
			clerkUserID); err != nil {
			return fmt.Errorf("record clerk tombstone: %w", err)
		}
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
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
	return tx.Commit()
}
