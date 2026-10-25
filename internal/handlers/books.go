package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"pal/internal/models"
	"pal/internal/store"

	"golang.org/x/text/unicode/norm"
)

type pageData struct {
	ActivePage      string
	Genres          []models.Genre
	GenreFilterID   int64
	GenreFilterName string
	StatusFilter    models.ReadingStatus
	NextPal         []bookCardData
	NextPalSections []bookSection
	NextPalCount    int
	Books           []bookCardData
	Sections        []bookSection
	TitleGroups     []titleGroup
	StatusCounts    []statusCount
	TotalCount      int
	Search          searchData
	SearchQuery     string
	SearchProvider  string
}

type bookSection struct {
	Label string
	Class string
	Books []bookCardData
}

type titleGroup struct {
	Letter string
	Books  []bookCardData
}

type statusCount struct {
	Status models.ReadingStatus
	Label  string
	Class  string
	Count  int
}

type bookCardData struct {
	Book            models.Book
	Genres          []models.Genre
	SelectedGenreID int64
}

var bookSectionDefinitions = []struct {
	status models.ReadingStatus
	label  string
	class  string
}{
	{models.StatusInProgress, "EN COURS", "in-progress"},
	{models.StatusToRead, "À LIRE", "to-read"},
	{models.StatusToBuy, "À ACHETER", "to-buy"},
	{models.StatusFinished, "TERMINÉ", "finished"},
}

func makePageData(books []models.Book, genres []models.Genre) pageData {
	data := pageData{Genres: genres}

	for _, book := range books {
		card := makeBookCard(book, genres)
		data.Books = append(data.Books, card)

		if book.IsNextPal {
			data.NextPal = append(data.NextPal, card)
		}
	}

	data.Sections = makeBookSections(data.Books)
	data.NextPalSections = makeBookSections(data.NextPal)
	data.NextPalCount = len(data.NextPal)

	return data
}

func makeBookSections(books []bookCardData) []bookSection {
	sections := make([]bookSection, 0, len(bookSectionDefinitions))

	for _, definition := range bookSectionDefinitions {
		section := bookSection{
			Label: definition.label,
			Class: definition.class,
		}

		for _, book := range books {
			if book.Book.ReadingStatus == definition.status {
				section.Books = append(section.Books, book)
			}
		}

		if len(section.Books) > 0 {
			sections = append(sections, section)
		}
	}

	return sections
}

// makeTitleGroups sorts books alphabetically by title, ignoring accents,
// then groups them by the first letter.
func makeTitleGroups(books []bookCardData) []titleGroup {
	sortedBooks := append([]bookCardData(nil), books...)

	sort.SliceStable(sortedBooks, func(i, j int) bool {
		return titleSortKey(sortedBooks[i].Book.Title) < titleSortKey(sortedBooks[j].Book.Title)
	})

	groups := make([]titleGroup, 0)

	for _, book := range sortedBooks {
		letter := firstLetter(book.Book.Title)

		if len(groups) == 0 || groups[len(groups)-1].Letter != letter {
			groups = append(groups, titleGroup{Letter: letter})
		}

		last := &groups[len(groups)-1]
		last.Books = append(last.Books, book)
	}

	return groups
}

// titleSortKey returns a normalized version of a title for sorting.
// Accents and other diacritical marks are ignored.
func titleSortKey(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}

	normalized := norm.NFD.String(strings.ToLower(title))

	return strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}

		return r
	}, normalized)
}

func firstLetter(title string) string {
	key := titleSortKey(title)

	if key == "" {
		return "#"
	}

	return strings.ToUpper(string([]rune(key)[0]))
}

// makeStatusCounts returns the count of books per reading status, used for
// the filter pills on the library page. Always returns all four statuses,
// even when their count is zero.
func makeStatusCounts(books []bookCardData) []statusCount {
	counts := make([]statusCount, 0, len(bookSectionDefinitions))

	for _, definition := range bookSectionDefinitions {
		count := 0

		for _, book := range books {
			if book.Book.ReadingStatus == definition.status {
				count++
			}
		}

		counts = append(counts, statusCount{
			Status: definition.status,
			Label:  definition.label,
			Class:  definition.class,
			Count:  count,
		})
	}

	return counts
}

func makeBookCard(book models.Book, genres []models.Genre) bookCardData {
	selectedGenreID := int64(0)

	if book.GenreID != nil {
		selectedGenreID = *book.GenreID
	}

	return bookCardData{
		Book:            book,
		Genres:          genres,
		SelectedGenreID: selectedGenreID,
	}
}

func (a *Application) home(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/search", http.StatusSeeOther)
}

func (a *Application) pageData(ctx context.Context) (pageData, error) {
	books, err := a.Store.ListBooks(ctx)
	if err != nil {
		return pageData{}, err
	}

	genres, err := a.Store.ListGenres(ctx)
	if err != nil {
		return pageData{}, err
	}

	return makePageData(books, genres), nil
}

func (a *Application) library(w http.ResponseWriter, r *http.Request) {
	genreID, err := optionalID(r.URL.Query().Get("genre"))
	if err != nil {
		http.Error(w, "genre invalide", http.StatusBadRequest)
		return
	}

	status := models.ReadingStatus(strings.TrimSpace(r.URL.Query().Get("status")))
	if status != "" && !models.IsValidReadingStatus(status) {
		http.Error(w, "statut invalide", http.StatusBadRequest)
		return
	}

	genres, err := a.Store.ListGenres(r.Context())
	if err != nil {
		http.Error(w, "impossible de charger les genres", http.StatusInternalServerError)
		return
	}

	// Books within the current genre scope, ignoring status — used to
	// compute the "Tous" total and the per-status pill counts.
	scopedBooks, err := a.Store.ListBooksFiltered(r.Context(), genreID, "")
	if err != nil {
		http.Error(w, "impossible de charger les livres", http.StatusInternalServerError)
		return
	}

	scopedCards := make([]bookCardData, 0, len(scopedBooks))

	for _, book := range scopedBooks {
		scopedCards = append(scopedCards, makeBookCard(book, genres))
	}

	// Books actually displayed below the toolbar (status filter applied too).
	displayedBooks := scopedBooks

	if status != "" {
		displayedBooks, err = a.Store.ListBooksFiltered(r.Context(), genreID, status)
		if err != nil {
			http.Error(w, "impossible de charger les livres", http.StatusInternalServerError)
			return
		}
	}

	data := pageData{
		Genres:       genres,
		ActivePage:   "library",
		StatusFilter: status,
	}

	for _, book := range displayedBooks {
		data.Books = append(data.Books, makeBookCard(book, genres))
	}

	data.TitleGroups = makeTitleGroups(data.Books)
	data.StatusCounts = makeStatusCounts(scopedCards)
	data.TotalCount = len(scopedCards)

	if genreID != nil {
		data.GenreFilterID = *genreID

		for _, genre := range genres {
			if genre.ID == *genreID {
				data.GenreFilterName = genre.Name
				break
			}
		}
	}

	a.render(w, http.StatusOK, "library", data)
}

func (a *Application) pal(w http.ResponseWriter, r *http.Request) {
	data, err := a.pageData(r.Context())
	if err != nil {
		http.Error(w, "impossible de charger les livres", http.StatusInternalServerError)
		return
	}

	data.ActivePage = "pal"

	a.render(w, http.StatusOK, "pal", data)
}

func (a *Application) createBook(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "requête invalide", http.StatusBadRequest)
		return
	}

	book := models.Book{
		OpenLibraryEditionID: strings.TrimSpace(r.FormValue("openlibrary_edition_id")),
		Title:                formatInput(r.FormValue("title")),
		Author:               formatInput(r.FormValue("author")),
		CoverURL:             strings.TrimSpace(r.FormValue("cover_url")),
	}

	if book.OpenLibraryEditionID == "" || book.Title == "" {
		http.Error(w, "identifiant et titre requis", http.StatusBadRequest)
		return
	}

	created, err := a.Store.CreateBook(r.Context(), book)
	if errors.Is(err, store.ErrBookExists) {
		a.render(w, http.StatusConflict, "error", "livre déjà présent")
		return
	}

	if err != nil {
		http.Error(w, "impossible d'ajouter le livre", http.StatusInternalServerError)
		return
	}

	if strings.TrimSpace(r.FormValue("add_to")) == "pal" {
		created, err = a.Store.ToggleNextPal(r.Context(), created.ID)
		if err != nil {
			http.Error(w, "livre ajouté, mais impossible de le mettre dans la PAL", http.StatusInternalServerError)
			return
		}
	}

	genres, err := a.Store.ListGenres(r.Context())
	if err != nil {
		http.Error(w, "impossible de charger les genres", http.StatusInternalServerError)
		return
	}

	a.render(w, http.StatusOK, "book-card", makeBookCard(created, genres))
}

func (a *Application) togglePal(w http.ResponseWriter, r *http.Request) {
	id, err := bookID(r)
	if err != nil {
		http.Error(w, "identifiant invalide", http.StatusBadRequest)
		return
	}

	book, err := a.Store.ToggleNextPal(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "livre introuvable", http.StatusNotFound)
		return
	}

	if err != nil {
		http.Error(w, "impossible de modifier le livre", http.StatusInternalServerError)
		return
	}

	a.render(w, http.StatusOK, "next-pal-button", book)
}

func (a *Application) updateStatus(w http.ResponseWriter, r *http.Request) {
	id, err := bookID(r)
	if err != nil {
		http.Error(w, "identifiant invalide", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "requête invalide", http.StatusBadRequest)
		return
	}

	status := models.ReadingStatus(strings.TrimSpace(r.FormValue("status")))

	_, err = a.Store.UpdateReadingStatus(r.Context(), id, status)
	if errors.Is(err, store.ErrInvalidReadingStatus) {
		http.Error(w, "statut invalide", http.StatusBadRequest)
		return
	}

	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "livre introuvable", http.StatusNotFound)
		return
	}

	if err != nil {
		http.Error(w, "impossible de modifier le statut", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *Application) updateGenre(w http.ResponseWriter, r *http.Request) {
	id, err := bookID(r)
	if err != nil {
		http.Error(w, "identifiant invalide", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "requête invalide", http.StatusBadRequest)
		return
	}

	genreID, err := optionalID(strings.TrimSpace(r.FormValue("genre_id")))
	if err != nil {
		http.Error(w, "genre invalide", http.StatusBadRequest)
		return
	}

	if err := a.Store.UpdateBookGenre(r.Context(), id, genreID); errors.Is(err, store.ErrGenreNotFound) {
		http.Error(w, "genre introuvable", http.StatusBadRequest)
		return
	} else if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "livre introuvable", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "impossible de modifier le genre", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *Application) deleteBook(w http.ResponseWriter, r *http.Request) {
	id, err := bookID(r)
	if err != nil {
		http.Error(w, "identifiant invalide", http.StatusBadRequest)
		return
	}

	if err := a.Store.DeleteBook(r.Context(), id); errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "livre introuvable", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "impossible de supprimer le livre", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func bookID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		return 0, errors.New("invalid book id")
	}

	return id, nil
}

func optionalID(value string) (*int64, error) {
	if value == "" {
		return nil, nil
	}

	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id < 1 {
		return nil, errors.New("invalid id")
	}

	return &id, nil
}

func formatInput(input string) string {
	return strings.Trim(strings.TrimSpace(input), "\"")
}
