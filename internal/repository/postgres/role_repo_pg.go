package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
)

type RoleRepository struct {
	db *sqlx.DB
}

func NewRoleRepo(db *sqlx.DB) *RoleRepository {
	return &RoleRepository{db: db}
}

func (r *RoleRepository) GetOrCreateRole(ctx context.Context, name, description string) (*domain.Role, error) {
	const query = `
        INSERT INTO role (role_name, description)
        VALUES ($1, $2)
        ON CONFLICT (role_name) DO UPDATE
        SET description = COALESCE(EXCLUDED.description, role.description),
            updated_at = NOW()
        RETURNING id, role_name, description, created_at, updated_at
    `
	row := r.db.QueryRowxContext(ctx, query, name, description)
	var role domain.Role
	if err := row.StructScan(&role); err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *RoleRepository) AssignUserRole(ctx context.Context, userID, roleID uuid.UUID) error {
	const query = `
        INSERT INTO user_role (role_id, user_id)
        VALUES ($1, $2)
        ON CONFLICT (role_id, user_id) DO NOTHING
    `
	_, err := r.db.ExecContext(ctx, query, roleID, userID)
	return err
}
