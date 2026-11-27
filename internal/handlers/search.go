package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"pal/internal/models"
)

type searchData struct {
	Existing []models.Book
	Results  []models.Book
	Error    string
}

type bookSearcher interface {
	Search(context.Context, string) ([]models.Book, error)
}

func (a *Application) searchProvider(name string) (bookSearcher, string, bool) {
	switch name {
	case "openlibrary":
		if a.OpenLibrary == nil {
			return nil, "Open Library", true
		}
		return a.OpenLibrary, "Open Library", true
	case "google_books":
		if a.GoogleBooks == nil {
			return nil, "Google Books", true
		}
		return a.GoogleBooks, "Google Books", true
	case "hardcover":
		if a.Hardcover == nil {
			return nil, "Hardcover", true
		}
		return a.Hardcover, "Hardcover", true
	default:
		return nil, "", false
	}
}

func (a *Application) searchPage(w http.ResponseWriter, r *http.Request) {
	genres, err := a.Store.ListGenres(r.Context())
	if err != nil {
		http.Error(w, "impossible de charger les genres", http.StatusInternalServerError)
		return
	}
	a.render(w, http.StatusOK, "search", pageData{
		ActivePage:     "search",
		Genres:         genres,
		SearchProvider: "openlibrary",
	})
}
func (a *Application) search(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "requête invalide", http.StatusBadRequest)
		return
	}

	isHTMX := r.Header.Get("HX-Request") == "true"
	provider := strings.TrimSpace(r.FormValue("provider"))
	if provider == "" {
		provider = "openlibrary"
	}
	searcher, label, ok := a.searchProvider(provider)
	if !ok {
		http.Error(w, "fournisseur invalide", http.StatusBadRequest)
		return
	}

	query := strings.TrimSpace(r.FormValue("q"))
	if query == "" {
		if isHTMX {
			w.WriteHeader(http.StatusOK)
			return
		}
		a.home(w, r)
		return
	}

	data := searchData{}
	existing, err := a.Store.SearchBooksByTitle(r.Context(), query)
	if err != nil {
		http.Error(w, "impossible de rechercher les livres", http.StatusInternalServerError)
		return
	}
	data.Existing = existing

	if searcher == nil {
		data.Error = fmt.Sprintf("Recherche %s indisponible", label)
	} else {
		books, err := searcher.Search(r.Context(), query)
		if err != nil {
			data.Error = fmt.Sprintf("Recherche %s indisponible : %v", label, err)
		} else {
			data.Results = books
		}
	}

	if isHTMX {
		a.render(w, http.StatusOK, "search-results", data)
		return
	}

	page, err := a.pageData(r.Context())
	if err != nil {
		http.Error(w, "impossible de charger les livres", http.StatusInternalServerError)
		return
	}
	page.SearchQuery = query
	page.Search = data
	page.ActivePage = "search"
	page.SearchProvider = provider
	a.render(w, http.StatusOK, "search", page)
}
