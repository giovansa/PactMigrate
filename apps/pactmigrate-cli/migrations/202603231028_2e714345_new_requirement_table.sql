-- PactMigrate migration: new_requirement_table
-- Edit the SQL below. After you change this file, run: make migration-rehash FILE=path/to/this/file.sql
-- (ContentHash in the filename must match SHA-256 of the file for WithVerifyContent(true).)
CREATE TABLE IF NOT EXISTS document_requirements(
    id VARCHAR(30) PRIMARY KEY,
    label VARCHAR(50) NOT NULL,
    identity_access_code VARCHAR(30) NOT NULL,
    position INT DEFAULT 0,
    description VARCHAR(100) DEFAULT '',
    type VARCHAR(30) NOT NULL,
    required_by_operator BOOLEAN NOT NULL,
    required_by_user BOOLEAN NOT NULL,
    is_deleted BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
    );
