-- +goose Up
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    username VARCHAR(64) NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT users_id_not_blank CHECK (length(btrim(id)) > 0),
    CONSTRAINT users_username_not_blank CHECK (length(btrim(username)) > 0)
);
COMMENT ON COLUMN users.password_hash IS 'Password hash only; no plaintext passwords';

-- +goose Down
DROP TABLE users;
