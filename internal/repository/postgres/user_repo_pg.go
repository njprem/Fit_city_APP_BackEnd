package postgres

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
)

type UserRepository struct {
	db *sqlx.DB
}

func NewUserRepo(db *sqlx.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) CreateEmailUser(ctx context.Context, email string, passwordHash, passwordSalt []byte) (*domain.User, error) {
	const query = `
        INSERT INTO user_account (email, password_hash, password_salt)
        VALUES ($1, $2, $3)
        RETURNING id, email, username, full_name, user_image_url,
                  password_hash, password_salt, profile_completed, created_at, updated_at
    `

	row := r.db.QueryRowxContext(ctx, query, email, passwordHash, passwordSalt)
	var user domain.User
	if err := row.StructScan(&user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) UpsertGoogleUser(ctx context.Context, email string, fullName *string, imageURL *string) (*domain.User, error) {
	const query = `
        INSERT INTO user_account (email, full_name, user_image_url, profile_completed)
        VALUES ($1, $2, $3, FALSE)
        ON CONFLICT (email) DO UPDATE
        SET full_name = COALESCE(EXCLUDED.full_name, user_account.full_name),
            user_image_url = CASE
                WHEN user_account.profile_completed THEN user_account.user_image_url
                ELSE COALESCE(EXCLUDED.user_image_url, user_account.user_image_url)
            END,
            profile_completed = user_account.profile_completed OR EXCLUDED.profile_completed,
            updated_at = NOW()
        RETURNING id, email, username, full_name, user_image_url,
                  password_hash, password_salt, profile_completed, created_at, updated_at
    `
	row := r.db.QueryRowxContext(ctx, query, email, fullName, imageURL)
	var user domain.User
	if err := row.StructScan(&user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	const query = `
        SELECT ua.id, ua.email, ua.username, ua.full_name, ua.user_image_url,
               ua.password_hash, ua.password_salt, ua.profile_completed, ua.created_at, ua.updated_at
        FROM user_account ua
        WHERE email = $1
    `
	var user domain.User
	if err := r.db.GetContext(ctx, &user, query, email); err != nil {
		return nil, err
	}
	if err := r.attachRoles(ctx, []*domain.User{&user}); err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	const query = `
        SELECT ua.id, ua.email, ua.username, ua.full_name, ua.user_image_url,
               ua.password_hash, ua.password_salt, ua.profile_completed, ua.created_at, ua.updated_at
        FROM user_account ua
        WHERE id = $1
    `
	var user domain.User
	if err := r.db.GetContext(ctx, &user, query, id); err != nil {
		return nil, err
	}
	if err := r.attachRoles(ctx, []*domain.User{&user}); err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) ListByIDs(ctx context.Context, ids []uuid.UUID) ([]domain.User, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	query, args, err := sqlx.In(`
        SELECT ua.id, ua.email, ua.username, ua.full_name, ua.user_image_url,
               ua.profile_completed, ua.created_at, ua.updated_at
        FROM user_account ua
        WHERE ua.id IN (?)
    `, ids)
	if err != nil {
		return nil, err
	}
	query = r.db.Rebind(query)

	users := make([]domain.User, 0, len(ids))
	if err := r.db.SelectContext(ctx, &users, query, args...); err != nil {
		return nil, err
	}
	userPtrs := make([]*domain.User, 0, len(users))
	for i := range users {
		userPtrs = append(userPtrs, &users[i])
	}
	if err := r.attachRoles(ctx, userPtrs); err != nil {
		return nil, err
	}
	return users, nil
}

func (r *UserRepository) UpdateProfile(ctx context.Context, id uuid.UUID, fullName *string, username *string, imageURL *string, profileCompleted bool) (*domain.User, error) {
	const query = `
        UPDATE user_account
        SET full_name = COALESCE($2, full_name),
            username = COALESCE($3, username),
            user_image_url = COALESCE($4, user_image_url),
            profile_completed = $5,
            updated_at = NOW()
        WHERE id = $1
        RETURNING id, email, username, full_name, user_image_url,
                  password_hash, password_salt, profile_completed, created_at, updated_at
    `
	row := r.db.QueryRowxContext(ctx, query, id, fullName, username, imageURL, profileCompleted)
	var user domain.User
	if err := row.StructScan(&user); err != nil {
		return nil, err
	}
	if err := r.attachRoles(ctx, []*domain.User{&user}); err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash, passwordSalt []byte) error {
	const query = `
        UPDATE user_account
        SET password_hash = $2,
            password_salt = $3,
            updated_at = NOW()
        WHERE id = $1
    `
	_, err := r.db.ExecContext(ctx, query, id, passwordHash, passwordSalt)
	return err
}

func (r *UserRepository) List(ctx context.Context, limit, offset int) ([]domain.User, error) {
	const query = `
        SELECT ua.id, ua.email, ua.username, ua.full_name, ua.user_image_url,
               ua.profile_completed, ua.created_at, ua.updated_at
        FROM user_account ua
        ORDER BY ua.created_at DESC
        LIMIT $1
        OFFSET $2
    `
	users := make([]domain.User, 0)
	if err := r.db.SelectContext(ctx, &users, query, limit, offset); err != nil {
		return nil, err
	}
	userPtrs := make([]*domain.User, 0, len(users))
	for i := range users {
		userPtrs = append(userPtrs, &users[i])
	}
	if err := r.attachRoles(ctx, userPtrs); err != nil {
		return nil, err
	}
	return users, nil
}

func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	steps := []struct {
		query string
		args  []any
	}{
		{query: `UPDATE role_change_handler SET editor = NULL WHERE editor = $1`, args: []any{id}},
		{query: `UPDATE user_role_change_handler SET editor = NULL WHERE editor = $1`, args: []any{id}},
		{query: `DELETE FROM review WHERE user_id = $1`, args: []any{id}},
		{query: `DELETE FROM favorite_list WHERE user_account_id = $1`, args: []any{id}},
		{query: `DELETE FROM sessions WHERE user_id = $1`, args: []any{id}},
		{query: `DELETE FROM user_role WHERE user_id = $1`, args: []any{id}},
		{query: `DELETE FROM password_reset WHERE user_id = $1`, args: []any{id}},
	}

	for _, step := range steps {
		if _, err = tx.ExecContext(ctx, step.query, step.args...); err != nil {
			return err
		}
	}

	result, err := tx.ExecContext(ctx, `DELETE FROM user_account WHERE id = $1`, id)
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

	if err = tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (r *UserRepository) attachRoles(ctx context.Context, users []*domain.User) error {
	if len(users) == 0 {
		return nil
	}

	userBuckets := make(map[uuid.UUID][]*domain.User)
	ids := make([]any, 0, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}
		user.Roles = nil
		if _, exists := userBuckets[user.ID]; !exists {
			ids = append(ids, user.ID)
		}
		userBuckets[user.ID] = append(userBuckets[user.ID], user)
	}

	if len(ids) == 0 {
		return nil
	}

	query, args, err := sqlx.In(`
        SELECT ur.user_id, r.id, r.role_name, r.description, r.created_at, r.updated_at
        FROM user_role ur
        JOIN role r ON r.id = ur.role_id
        WHERE ur.user_id IN (?)
        ORDER BY r.created_at
    `, ids)
	if err != nil {
		return err
	}
	query = r.db.Rebind(query)

	rows := make([]struct {
		UserID uuid.UUID `db:"user_id"`
		domain.Role
	}, 0)
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return err
	}

	for _, row := range rows {
		if bucket, ok := userBuckets[row.UserID]; ok {
			for _, user := range bucket {
				user.Roles = append(user.Roles, row.Role)
			}
		}
	}

	return nil
}
