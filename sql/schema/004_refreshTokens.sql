-- +goose Up
CREATE TABLE refresh_tokens(
    token TEXT NOT NULL,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at  TIMESTAMP NOT NULL,
    revoked_at TIMESTAMP
);

-- +goose Down
DROP TABLE refresh_tokens;