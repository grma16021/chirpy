-- +goose Up
CREATE TABLE users(
    id UUID UNIQUE PRIMARY KEY,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    email TEXT UNIQUE NOT NULL
);


-- postgres://postgres:@localhost:5432/chirpy

-- +goose Down
DROP TABLE users;