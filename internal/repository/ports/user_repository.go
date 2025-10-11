package ports

import (
	"context"

	"github.com/google/uuid"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
)

type UserRepository interface {
	CreateEmailUser(ctx context.Context, email string, passwordHash, passwordSalt []byte) (*domain.User, error)
	UpsertGoogleUser(ctx context.Context, email string, fullName *string, imageURL *string) (*domain.User, error)
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	UpdateProfile(ctx context.Context, id uuid.UUID, fullName *string, username *string, imageURL *string, profileCompleted bool) (*domain.User, error)
	UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash, passwordSalt []byte) error
	List(ctx context.Context, limit, offset int) ([]domain.User, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
