package httpui

import (
	"io/fs"
	"net/http"
)

func (h *Handler) RootHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/workbench", http.StatusTemporaryRedirect)
}

func (h *Handler) WorkbenchHandler(w http.ResponseWriter, r *http.Request) {
	data, err := assets.ReadFile("assets/workbench.html")
	if err != nil {
		http.Error(w, "工作台资源不可用", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func (h *Handler) AssetHandler(w http.ResponseWriter, r *http.Request) {
	name := "assets/app.css"
	contentType := "text/css; charset=utf-8"
	if r.URL.Path == "/assets/app.js" {
		name = "assets/app.js"
		contentType = "text/javascript; charset=utf-8"
	}
	data, err := fs.ReadFile(assets, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write(data)
}
