CREATE TABLE IF NOT EXISTS review (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES user_account(id),
    destination_id UUID NOT NULL REFERENCES travel_destination(id),
    rating SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    title TEXT,
    content TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS review_unique_user_destination_idx
    ON review(user_id, destination_id);

CREATE TABLE IF NOT EXISTS favorite_list (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_account_id UUID NOT NULL REFERENCES user_account(id),
    destination_id UUID REFERENCES travel_destination(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS favorite_user_destination_idx
    ON favorite_list(user_account_id, destination_id);

CREATE TABLE IF NOT EXISTS sessions (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES user_account(id),
    token TEXT UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE INDEX IF NOT EXISTS sessions_user_idx ON sessions(user_id);
