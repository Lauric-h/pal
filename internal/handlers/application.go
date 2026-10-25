package handlers

import (
	"bytes"
	"html/template"
	"net/http"

	"pal/internal/googlebooks"
	"pal/internal/openlibrary"
	"pal/internal/store"
)

type Application struct {
	Store       *store.Store
	OpenLibrary *openlibrary.Client
	GoogleBooks *googlebooks.Client
	Templates   *template.Template
}

func (a *Application) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", a.home)
	mux.HandleFunc("GET /search", a.searchPage)
	mux.HandleFunc("POST /search", a.search)
	mux.HandleFunc("GET /pal", a.pal)
	mux.HandleFunc("GET /library", a.library)
	mux.HandleFunc("POST /books", a.createBook)
	mux.HandleFunc("POST /books/{id}/toggle-pal", a.togglePal)
	mux.HandleFunc("POST /books/{id}/status", a.updateStatus)
	mux.HandleFunc("POST /books/{id}/progress", a.updateProgress)
	mux.HandleFunc("DELETE /books/{id}", a.deleteBook)
	return mux
}

func (a *Application) render(w http.ResponseWriter, status int, name string, data any) {
	var body bytes.Buffer
	if err := a.Templates.ExecuteTemplate(&body, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body.Bytes())
}
