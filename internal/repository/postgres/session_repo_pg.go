package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
)

type SessionRepository struct {
	db *sqlx.DB
}

func NewSessionRepo(db *sqlx.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) CreateSession(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) (*domain.Session, error) {
	const query = `
        INSERT INTO sessions (user_id, token, expires_at, is_active)
        VALUES ($1, $2, $3, true)
        RETURNING id, user_id, token, created_at, expires_at, is_active
    `
	row := r.db.QueryRowxContext(ctx, query, userID, token, expiresAt)
	var session domain.Session
	if err := row.StructScan(&session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *SessionRepository) DeactivateSession(ctx context.Context, token string) error {
	const query = `
        UPDATE sessions SET is_active = false, expires_at = NOW()
        WHERE token = $1 AND is_active = true
    `
	_, err := r.db.ExecContext(ctx, query, token)
	return err
}

func (r *SessionRepository) FindActiveSession(ctx context.Context, token string) (*domain.Session, error) {
	const query = `
        SELECT id, user_id, token, created_at, expires_at, is_active
        FROM sessions
        WHERE token = $1 AND is_active = true AND expires_at > NOW()
    `
	var session domain.Session
	if err := r.db.GetContext(ctx, &session, query, token); err != nil {
		return nil, err
	}
	return &session, nil
}
