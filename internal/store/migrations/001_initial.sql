CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL,
    username_normalized TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin', 'user')),
    state TEXT NOT NULL DEFAULT 'active' CHECK (state IN ('active', 'disabled', 'archived')),
    stored_file_limit_bytes INTEGER NOT NULL CHECK (stored_file_limit_bytes BETWEEN 1000000 AND 500000000),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    archived_at TEXT
);

CREATE UNIQUE INDEX IF NOT EXISTS users_one_admin
ON users(role) WHERE role = 'admin';

CREATE TABLE IF NOT EXISTS folders (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    owner_user_id INTEGER NOT NULL REFERENCES users(id),
    parent_folder_id INTEGER REFERENCES folders(id),
    name TEXT NOT NULL,
    name_normalized TEXT NOT NULL,
    is_root INTEGER NOT NULL DEFAULT 0 CHECK (is_root IN (0, 1)),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT,
    original_parent_folder_id INTEGER
);

CREATE UNIQUE INDEX IF NOT EXISTS folders_one_root_per_user
ON folders(owner_user_id) WHERE is_root = 1;

CREATE UNIQUE INDEX IF NOT EXISTS folders_unique_active_sibling_name
ON folders(owner_user_id, COALESCE(parent_folder_id, 0), name_normalized)
WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS clips (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    owner_user_id INTEGER NOT NULL REFERENCES users(id),
    parent_folder_id INTEGER NOT NULL REFERENCES folders(id),
    public_id TEXT NOT NULL UNIQUE,
    storage_id TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    title_normalized TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('uploading', 'queued', 'validating', 'processing', 'ready', 'failed', 'cancelled', 'unavailable')),
    size_bytes INTEGER,
    duration_seconds REAL,
    width INTEGER,
    height INTEGER,
    frame_rate REAL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT,
    original_parent_folder_id INTEGER
);

CREATE UNIQUE INDEX IF NOT EXISTS clips_unique_active_sibling_title
ON clips(parent_folder_id, title_normalized)
WHERE deleted_at IS NULL AND state NOT IN ('failed', 'cancelled');

CREATE INDEX IF NOT EXISTS clips_public_lookup ON clips(public_id, state, deleted_at);

CREATE TABLE IF NOT EXISTS jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    clip_id INTEGER NOT NULL REFERENCES clips(id),
    state TEXT NOT NULL CHECK (state IN ('queued', 'validating', 'processing', 'ready', 'failed', 'cancelled')),
    progress INTEGER NOT NULL DEFAULT 0 CHECK (progress BETWEEN 0 AND 100),
    error_code TEXT,
    error_message TEXT,
    claimed_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS jobs_state_order ON jobs(state, created_at, id);

CREATE TABLE IF NOT EXISTS storage_reservations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id INTEGER REFERENCES jobs(id),
    reservation_key TEXT NOT NULL UNIQUE,
    reserved_bytes INTEGER NOT NULL CHECK (reserved_bytes > 0),
    created_at TEXT NOT NULL
);
