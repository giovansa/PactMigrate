-- PactMigrate migration: new_profile
-- Edit the SQL below. After you change this file, run: make migration-rehash FILE=path/to/this/file.sql
-- (ContentHash in the filename must match SHA-256 of the file for WithVerifyContent(true).)
CREATE TABLE IF NOT EXISTS profiles (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(150) NOT NULL UNIQUE,
    picture varchar(150) NOT NULL DEFAULT '',
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
