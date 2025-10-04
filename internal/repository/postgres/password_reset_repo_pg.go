package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
)

type PasswordResetRepository struct {
	db *sqlx.DB
}

func NewPasswordResetRepo(db *sqlx.DB) *PasswordResetRepository {
	return &PasswordResetRepository{db: db}
}

func (r *PasswordResetRepository) ConsumeByUser(ctx context.Context, userID uuid.UUID) error {
	const query = `
        UPDATE password_reset
        SET consumed = TRUE,
            updated_at = NOW()
        WHERE user_id = $1 AND consumed = FALSE
    `
	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}

func (r *PasswordResetRepository) Create(ctx context.Context, userID uuid.UUID, otpHash, otpSalt []byte, expiresAt time.Time) (*domain.PasswordReset, error) {
	const query = `
        INSERT INTO password_reset (user_id, otp_hash, otp_salt, expires_at)
        VALUES ($1, $2, $3, $4)
        RETURNING id, user_id, otp_hash, otp_salt, expires_at, consumed, created_at
    `
	row := r.db.QueryRowxContext(ctx, query, userID, otpHash, otpSalt, expiresAt)
	var reset domain.PasswordReset
	if err := row.StructScan(&reset); err != nil {
		return nil, err
	}
	return &reset, nil
}

func (r *PasswordResetRepository) FindActiveByUser(ctx context.Context, userID uuid.UUID, now time.Time) (*domain.PasswordReset, error) {
	const query = `
        SELECT id, user_id, otp_hash, otp_salt, expires_at, consumed, created_at
        FROM password_reset
        WHERE user_id = $1 AND consumed = FALSE AND expires_at >= $2
        ORDER BY created_at DESC
        LIMIT 1
    `
	var reset domain.PasswordReset
	if err := r.db.GetContext(ctx, &reset, query, userID, now); err != nil {
		return nil, err
	}
	return &reset, nil
}

func (r *PasswordResetRepository) MarkConsumed(ctx context.Context, id int64) error {
	const query = `
        UPDATE password_reset
        SET consumed = TRUE,
            updated_at = NOW()
        WHERE id = $1
    `
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}
