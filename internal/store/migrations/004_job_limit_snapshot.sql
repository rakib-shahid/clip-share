ALTER TABLE jobs ADD COLUMN target_size_bytes INTEGER;
ALTER TABLE jobs ADD COLUMN stored_limit_bypassed INTEGER NOT NULL DEFAULT 0 CHECK (stored_limit_bypassed IN (0, 1));
