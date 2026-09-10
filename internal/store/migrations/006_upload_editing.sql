CREATE TABLE upload_sessions (
    id TEXT PRIMARY KEY,
    clip_id INTEGER NOT NULL UNIQUE REFERENCES clips(id) ON DELETE CASCADE,
    acting_user_id INTEGER NOT NULL REFERENCES users(id),
    reservation_key TEXT NOT NULL UNIQUE,
    state TEXT NOT NULL CHECK (state IN ('editing_session', 'finalizing')),
    revision INTEGER NOT NULL DEFAULT 1,
    trim_start_ms INTEGER NOT NULL DEFAULT 0,
    trim_end_ms INTEGER NOT NULL,
    source_size_bytes INTEGER NOT NULL,
    source_duration_ms INTEGER NOT NULL,
    video_stream_index INTEGER NOT NULL,
    compression_requested INTEGER NOT NULL DEFAULT 0 CHECK (compression_requested IN (0, 1)),
    quality_crf INTEGER NOT NULL,
    max_height INTEGER NOT NULL,
    target_size_bytes INTEGER NOT NULL,
    stored_limit_bypassed INTEGER NOT NULL DEFAULT 0 CHECK (stored_limit_bypassed IN (0, 1)),
    source_container TEXT NOT NULL,
    source_video_codec TEXT NOT NULL,
    source_audio_codec TEXT,
    source_width INTEGER NOT NULL,
    source_height INTEGER NOT NULL,
    source_frame_rate REAL NOT NULL,
    source_pixel_format TEXT,
    source_profile TEXT,
    final_job_id INTEGER UNIQUE REFERENCES jobs(id),
    last_heartbeat_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE UNIQUE INDEX upload_sessions_one_active_actor
ON upload_sessions(acting_user_id) WHERE state IN ('editing_session', 'finalizing');

CREATE INDEX upload_sessions_expiry ON upload_sessions(state, last_heartbeat_at);

CREATE TABLE upload_session_audio (
    session_id TEXT NOT NULL REFERENCES upload_sessions(id) ON DELETE CASCADE,
    stream_index INTEGER NOT NULL,
    codec TEXT NOT NULL,
    channels INTEGER NOT NULL DEFAULT 0,
    channel_layout TEXT,
    sample_rate INTEGER NOT NULL DEFAULT 0,
    language TEXT,
    is_default INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0, 1)),
    usable INTEGER NOT NULL DEFAULT 1 CHECK (usable IN (0, 1)),
    unusable_reason TEXT,
    included INTEGER NOT NULL DEFAULT 0 CHECK (included IN (0, 1)),
    gain_db REAL NOT NULL DEFAULT 0,
    PRIMARY KEY(session_id, stream_index)
);
