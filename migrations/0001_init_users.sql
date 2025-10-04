CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS role (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    role_name TEXT UNIQUE NOT NULL,
    description TEXT,
    draft UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_account (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username TEXT UNIQUE,
    full_name TEXT,
    user_image_url TEXT,
    email TEXT UNIQUE NOT NULL,
    role_id UUID NOT NULL REFERENCES role(id),
    password_hash BYTEA,
    password_salt BYTEA,
    profile_completed BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS role_change_handler (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    role_id UUID,
    role_name TEXT,
    description TEXT,
    editor UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_role (
    role_id UUID NOT NULL,
    user_id UUID NOT NULL,
    draft UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (role_id, user_id)
);

CREATE TABLE IF NOT EXISTS user_role_change_handler (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    role_id UUID,
    editor UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE role
    ADD CONSTRAINT role_draft_fk FOREIGN KEY (draft) REFERENCES role_change_handler(id);

ALTER TABLE role_change_handler
    ADD CONSTRAINT role_change_role_fk FOREIGN KEY (role_id) REFERENCES role(id);

ALTER TABLE role_change_handler
    ADD CONSTRAINT role_change_editor_fk FOREIGN KEY (editor) REFERENCES user_account(id);

ALTER TABLE user_role
    ADD CONSTRAINT user_role_role_fk FOREIGN KEY (role_id) REFERENCES role(id);

ALTER TABLE user_role
    ADD CONSTRAINT user_role_user_fk FOREIGN KEY (user_id) REFERENCES user_account(id);

ALTER TABLE user_role
    ADD CONSTRAINT user_role_draft_fk FOREIGN KEY (draft) REFERENCES user_role_change_handler(id);

ALTER TABLE user_role_change_handler
    ADD CONSTRAINT user_role_change_role_fk FOREIGN KEY (role_id) REFERENCES role(id);

ALTER TABLE user_role_change_handler
    ADD CONSTRAINT user_role_change_editor_fk FOREIGN KEY (editor) REFERENCES user_account(id);

INSERT INTO role (role_name, description)
VALUES ('user', 'Default application role for authenticated users')
ON CONFLICT (role_name) DO NOTHING;
