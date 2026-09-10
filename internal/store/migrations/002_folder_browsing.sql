CREATE INDEX IF NOT EXISTS folders_active_parent_order
ON folders(parent_folder_id, name_normalized, id)
WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS clips_active_parent_order
ON clips(parent_folder_id, created_at DESC, id DESC)
WHERE deleted_at IS NULL;
