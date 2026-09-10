package httpapi

import (
	"net/http"
	"unicode/utf8"

	"clip-share/internal/library"
)

func (a *API) search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if utf8.RuneCountInString(query) > 200 {
		writeError(w, http.StatusBadRequest, "search_query_too_long", "Search queries cannot exceed 200 characters.", "q")
		return
	}
	results, err := a.store.Search(r.Context(), library.NormalizeSearchQuery(query), requiredOwner(userFromContext(r.Context())))
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, results)
}
