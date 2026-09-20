package httpapi

import (
	"net/http"
	"strconv"
	"strings"
)

func (a *API) getJobStatuses(w http.ResponseWriter, r *http.Request) {
	ids, ok := parseJobIDs(r.URL.Query()["ids"])
	if !ok {
		invalidJobIDs(w)
		return
	}
	actor := userFromContext(r.Context())
	jobs, err := a.store.JobStatuses(r.Context(), ids, requiredOwner(actor))
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}

func parseJobIDs(values []string) ([]int64, bool) {
	if len(values) != 1 || values[0] == "" {
		return nil, false
	}
	parts := strings.Split(values[0], ",")
	if len(parts) < 1 || len(parts) > 100 {
		return nil, false
	}
	ids := make([]int64, 0, len(parts))
	seen := make(map[int64]struct{}, len(parts))
	for _, part := range parts {
		if part == "" || strings.TrimSpace(part) != part {
			return nil, false
		}
		for _, digit := range part {
			if digit < '0' || digit > '9' {
				return nil, false
			}
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil || id < 1 {
			return nil, false
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, false
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, true
}

func invalidJobIDs(w http.ResponseWriter) {
	writeError(w, http.StatusBadRequest, "invalid_job_ids", "Choose between 1 and 100 unique positive job IDs.", "ids")
}
