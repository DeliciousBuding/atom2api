package main

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed frontend/dist/*
var frontendFS embed.FS

func serveFrontend() http.Handler {
	distFS, _ := fs.Sub(frontendFS, "frontend/dist")
	return spaHandler{fs: http.FS(distFS)}
}

type spaHandler struct {
	fs http.FileSystem
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/admin/")
	if path == "" {
		path = "index.html"
	}
	f, err := h.fs.Open(path)
	if err != nil {
		http.FileServer(h.fs).ServeHTTP(w, r)
		return
	}
	f.Close()
	http.StripPrefix("/admin/", http.FileServer(h.fs)).ServeHTTP(w, r)
}
