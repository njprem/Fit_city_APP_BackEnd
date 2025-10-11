package ports

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
)

type SessionRepository interface {
	CreateSession(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) (*domain.Session, error)
	DeactivateSession(ctx context.Context, token string) error
	FindActiveSession(ctx context.Context, token string) (*domain.Session, error)
}
