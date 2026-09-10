ALTER TABLE jobs ADD COLUMN compression_requested INTEGER NOT NULL DEFAULT 0 CHECK (compression_requested IN (0, 1));
ALTER TABLE jobs ADD COLUMN quality_crf INTEGER NOT NULL DEFAULT 24 CHECK (quality_crf IN (18, 21, 24, 27, 30));
ALTER TABLE jobs ADD COLUMN max_height INTEGER NOT NULL DEFAULT 1080 CHECK (max_height IN (480, 720, 1080));
ALTER TABLE jobs ADD COLUMN source_size_bytes INTEGER;
ALTER TABLE jobs ADD COLUMN source_container TEXT;
ALTER TABLE jobs ADD COLUMN source_video_codec TEXT;
ALTER TABLE jobs ADD COLUMN source_audio_codec TEXT;
ALTER TABLE jobs ADD COLUMN source_duration_seconds REAL;
ALTER TABLE jobs ADD COLUMN source_width INTEGER;
ALTER TABLE jobs ADD COLUMN source_height INTEGER;
ALTER TABLE jobs ADD COLUMN source_frame_rate REAL;

CREATE INDEX IF NOT EXISTS storage_reservations_active
ON storage_reservations(created_at, reserved_bytes);
