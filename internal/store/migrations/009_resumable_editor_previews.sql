DROP INDEX IF EXISTS upload_sessions_one_active_actor;

ALTER TABLE upload_sessions ADD COLUMN preview_rendering INTEGER NOT NULL DEFAULT 0
    CHECK (preview_rendering IN (0, 1));
ALTER TABLE upload_sessions ADD COLUMN pending_preview_start_ms INTEGER;
ALTER TABLE upload_sessions ADD COLUMN pending_preview_end_ms INTEGER;
ALTER TABLE upload_sessions ADD COLUMN pending_preview_revision INTEGER;

-- A render interrupted by the deployment cannot still be running. Preserve an
-- older ready asset when present; otherwise expose the interrupted attempt as a
-- failed preview that the user can retry.
UPDATE upload_sessions
SET preview_state = 'failed'
WHERE preview_state = 'rendering';

CREATE INDEX upload_sessions_resumable
ON upload_sessions(acting_user_id, state, last_heartbeat_at);
