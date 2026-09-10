ALTER TABLE upload_sessions ADD COLUMN preview_state TEXT NOT NULL DEFAULT 'none' CHECK (preview_state IN ('none','rendering','ready','failed'));
ALTER TABLE upload_sessions ADD COLUMN preview_start_ms INTEGER;
ALTER TABLE upload_sessions ADD COLUMN preview_end_ms INTEGER;
ALTER TABLE upload_sessions ADD COLUMN preview_revision INTEGER;
