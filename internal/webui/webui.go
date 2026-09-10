package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

// Docker replaces dist with the compiled React application before building Go.
//
//go:embed dist/*
var assets embed.FS

func Handler() http.Handler {
	dist, err := fs.Sub(assets, "dist")
	if err != nil {
		panic(err)
	}
	files := http.FileServer(http.FS(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if _, err := fs.Stat(dist, path); err == nil {
				files.ServeHTTP(w, r)
				return
			}
		}
		r.URL.Path = "/"
		w.Header().Set("Cache-Control", "no-store")
		files.ServeHTTP(w, r)
	})
}
