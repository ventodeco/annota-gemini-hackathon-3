package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/gemini-hackathon/app/internal/models"
	"github.com/gemini-hackathon/app/internal/storage"
)

// ErrVerifiedEmailRequired means the Clerk user has no verified primary email,
// so ANNOTA cannot create a profile for it.
var ErrVerifiedEmailRequired = errors.New("verified primary email required")

// IdentityStore is the subset of storage.DB used to map Clerk subjects.
type IdentityStore interface {
	GetUserByClerkID(ctx context.Context, clerkUserID string) (*models.User, error)
	IsClerkUserDeleted(ctx context.Context, clerkUserID string) (bool, error)
	ProvisionClerkUser(ctx context.Context, params storage.ClerkProvisionParams) (*models.User, error)
}

// Resolution is the local account state for a verified Clerk subject.
type Resolution struct {
	UserID int64
	// Deleted is true when the subject's ANNOTA account was deleted. Its
	// tombstone is permanent, so the profile is never recreated.
	Deleted bool
}

// IdentityResolver maps verified Clerk subjects to local numeric users,
// provisioning on the first authenticated request.
type IdentityResolver struct {
	store     IdentityStore
	directory IdentityDirectory
}

func NewIdentityResolver(store IdentityStore, directory IdentityDirectory) *IdentityResolver {
	return &IdentityResolver{store: store, directory: directory}
}

func (r *IdentityResolver) Resolve(ctx context.Context, clerkUserID string) (Resolution, error) {
	existing, err := r.store.GetUserByClerkID(ctx, clerkUserID)
	if err != nil {
		return Resolution{}, fmt.Errorf("lookup clerk user: %w", err)
	}
	if existing != nil {
		return Resolution{UserID: existing.ID}, nil
	}

	deleted, err := r.store.IsClerkUserDeleted(ctx, clerkUserID)
	if err != nil {
		return Resolution{}, fmt.Errorf("check clerk tombstone: %w", err)
	}
	if deleted {
		return Resolution{Deleted: true}, nil
	}

	identity, err := r.directory.GetIdentity(ctx, clerkUserID)
	if err != nil {
		return Resolution{}, err
	}
	if identity.VerifiedPrimaryEmail == "" {
		return Resolution{}, ErrVerifiedEmailRequired
	}

	provisioned, err := r.store.ProvisionClerkUser(ctx, storage.ClerkProvisionParams{
		ClerkUserID:   clerkUserID,
		Email:         identity.VerifiedPrimaryEmail,
		AvatarURL:     identity.AvatarURL,
		GoogleSubject: identity.GoogleSubject,
	})
	if errors.Is(err, storage.ErrClerkUserDeleted) {
		return Resolution{Deleted: true}, nil
	}
	if err != nil {
		return Resolution{}, err
	}
	return Resolution{UserID: provisioned.ID}, nil
}
