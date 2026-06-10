CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(64) NOT NULL,
    slug VARCHAR(80) UNIQUE,
    description TEXT,
    loader VARCHAR(16) NOT NULL,
    game_version VARCHAR(16) NOT NULL,
    loader_version VARCHAR(32),
    java_version INTEGER NOT NULL DEFAULT 17,
    jvm_args TEXT,
    icon_url TEXT,
    java_path_windows VARCHAR(512),
    java_path_linux VARCHAR(512),
    java_path_mac_os VARCHAR(512),
    launch_command_windows TEXT,
    launch_command_linux TEXT,
    launch_command_mac_os TEXT,
    manifest_version INTEGER NOT NULL DEFAULT 0,
    manifest_updated_at TIMESTAMPTZ,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
