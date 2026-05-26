package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

func Handler() http.Handler {
	dist, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	return spaHandler{
		files:      dist,
		fileServer: http.FileServer(http.FS(dist)),
	}
}

type spaHandler struct {
	files      fs.FS
	fileServer http.Handler
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "." || name == "" {
		h.fileServer.ServeHTTP(w, r)
		return
	}
	if stat, err := fs.Stat(h.files, name); err == nil && !stat.IsDir() {
		h.fileServer.ServeHTTP(w, r)
		return
	}

	next := r.Clone(r.Context())
	next.URL.Path = "/"
	h.fileServer.ServeHTTP(w, next)
}
