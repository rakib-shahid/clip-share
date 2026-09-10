package httpapi

import (
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"clip-share/internal/media"
)

var publicPage = template.Must(template.New("clip").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="robots" content="noindex,nofollow"><meta property="og:type" content="video.other"><meta property="og:title" content="{{.Title}}"><meta property="og:url" content="{{.URL}}"><meta property="og:video" content="{{.VideoURL}}"><meta property="og:video:secure_url" content="{{.VideoURL}}"><meta property="og:video:type" content="video/mp4"><meta property="og:image" content="{{.PosterURL}}"><meta name="twitter:card" content="player"><meta name="twitter:title" content="{{.Title}}"><title>{{.Title}} · Clip Share</title><style>html,body{margin:0;min-height:100%;background:#090d14;color:#e2e8f0;font:16px system-ui,sans-serif}main{max-width:1100px;margin:0 auto;padding:32px 20px}a{color:#7dd3fc;text-decoration:none}h1{font-size:clamp(1.4rem,3vw,2.4rem);margin:0 0 20px}video{display:block;width:100%;max-height:75vh;background:#000;border-radius:14px;box-shadow:0 20px 70px #0008}nav{margin-bottom:28px;display:flex;gap:18px}</style></head><body><main><nav><a href="/">Home</a><a href="/login">Login</a></nav><h1>{{.Title}}</h1><video controls playsinline preload="metadata" poster="{{.PosterURL}}" src="{{.VideoURL}}"></video></main></body></html>`))

type publicPageData struct{ Title, URL, VideoURL, PosterURL string }

func (a *API) publicClipPage(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/c/")
	clip, err := a.store.PublicClipByID(r.Context(), id)
	if err != nil {
		a.publicNotFound(w)
		return
	}
	base := strings.TrimRight(a.cfg.BaseURL, "/")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = publicPage.Execute(w, publicPageData{clip.Title, base + "/c/" + clip.PublicID, base + "/m/" + clip.PublicID + "/video", base + "/m/" + clip.PublicID + "/poster"})
}

func (a *API) publicMedia(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/m/"), "/")
	if len(parts) != 2 || (parts[1] != "video" && parts[1] != "poster") {
		a.publicNotFound(w)
		return
	}
	clip, err := a.store.PublicClipByID(r.Context(), parts[0])
	if err != nil {
		a.publicNotFound(w)
		return
	}
	name, contentType := "video.mp4", "video/mp4"
	if parts[1] == "poster" {
		name, contentType = "poster.jpg", "image/jpeg"
	}
	entry, err := a.store.MediaLayoutEntry(r.Context(), clip.ID)
	if err != nil {
		a.publicNotFound(w)
		return
	}
	directory, err := media.AssetDirectory(a.cfg.DataDir, entry)
	if err != nil {
		a.publicNotFound(w)
		return
	}
	file, err := os.Open(filepath.Join(directory, name))
	if err != nil {
		a.publicNotFound(w)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		a.publicNotFound(w)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.Header().Set("Content-Type", contentType)
	http.ServeContent(w, r, name, info.ModTime(), file)
}

func (a *API) publicNotFound(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	http.NotFound(w, nil)
}
