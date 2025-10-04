package ports

import (
	"context"

	"github.com/google/uuid"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
)

type RoleRepository interface {
	GetOrCreateRole(ctx context.Context, name, description string) (*domain.Role, error)
	AssignUserRole(ctx context.Context, userID, roleID uuid.UUID) error
}
