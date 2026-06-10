CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS game_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_id UUID REFERENCES profiles(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    path VARCHAR(512) NOT NULL,
    url TEXT NOT NULL,
    hash_sha256 VARCHAR(64) NOT NULL,
    size BIGINT NOT NULL,
    file_type VARCHAR(16) NOT NULL DEFAULT 'mod'
);

