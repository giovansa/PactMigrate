-- PactMigrate migration: new_user_table
-- Edit the SQL below. If you use WithVerifyContent(true), the filename hash must
-- match the SHA-256 of this file (re-run this script or rename after edits).
CREATE TABLE IF NOT EXISTS users_data (
    id SERIAL PRIMARY KEY,
    username VARCHAR(150) NOT NULL UNIQUE,
    fullname VARCHAR(150) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);

