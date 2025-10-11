package ports

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
)

type PasswordResetRepository interface {
	Create(ctx context.Context, userID uuid.UUID, otpHash, otpSalt []byte, expiresAt time.Time) (*domain.PasswordReset, error)
	FindActiveByUser(ctx context.Context, userID uuid.UUID, now time.Time) (*domain.PasswordReset, error)
	MarkConsumed(ctx context.Context, id int64) error
	ConsumeByUser(ctx context.Context, userID uuid.UUID) error
}
