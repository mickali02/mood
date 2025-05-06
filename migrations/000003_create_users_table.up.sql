-- migrations/000003_create_users_table.up.sql
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT NOW(),
    name TEXT NOT NULL,
    email CITEXT UNIQUE NOT NULL, -- Uses the citext extension
    password_hash BYTEA NOT NULL, -- Stores the hashed password
    activated BOOLEAN NOT NULL DEFAULT FALSE -- For account activation status
);