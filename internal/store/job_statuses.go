package store

import (
	"context"
	"database/sql"
	"strings"
)

// JobStatus is the deliberately small, safe projection used by explorer polling.
type JobStatus struct {
	JobID        int64   `json:"jobId"`
	ClipID       int64   `json:"clipId"`
	State        string  `json:"state"`
	Progress     *int    `json:"progress"`
	ErrorMessage *string `json:"errorMessage"`
	SizeBytes    *int64  `json:"sizeBytes"`
}

func (s *Store) JobStatuses(ctx context.Context, ids []int64, requiredOwnerID *int64) ([]JobStatus, error) {
	marks := make([]string, len(ids))
	args := make([]any, 0, len(ids)+1)
	for i, id := range ids {
		marks[i] = "?"
		args = append(args, id)
	}
	query := `SELECT j.id,j.clip_id,c.state,j.progress,j.error_message,c.size_bytes FROM jobs j JOIN clips c ON c.id=j.clip_id WHERE j.id IN (` + strings.Join(marks, ",") + `)`
	if requiredOwnerID != nil {
		query += ` AND c.owner_user_id=?`
		args = append(args, *requiredOwnerID)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byID := make(map[int64]JobStatus, len(ids))
	for rows.Next() {
		var status JobStatus
		var progress sql.NullInt64
		if err := rows.Scan(&status.JobID, &status.ClipID, &status.State, &progress, &status.ErrorMessage, &status.SizeBytes); err != nil {
			return nil, err
		}
		if progress.Valid {
			value := int(progress.Int64)
			status.Progress = &value
		}
		byID[status.JobID] = status
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result := make([]JobStatus, 0, len(byID))
	for _, id := range ids {
		if status, ok := byID[id]; ok {
			result = append(result, status)
		}
	}
	return result, nil
}
