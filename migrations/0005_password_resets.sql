CREATE TABLE IF NOT EXISTS password_reset (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES user_account(id) ON DELETE CASCADE,
    otp_hash BYTEA NOT NULL,
    otp_salt BYTEA NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_password_reset_user_active
    ON password_reset (user_id)
    WHERE consumed = FALSE;
