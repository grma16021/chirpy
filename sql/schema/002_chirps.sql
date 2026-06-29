-- +goose Up
CREATE TABLE chirps (
    id UUID UNIQUE PRIMARY KEY,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    body TEXT NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE chirps;