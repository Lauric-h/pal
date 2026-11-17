package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"pal/internal/models"

	_ "modernc.org/sqlite"
)

func TestBookProgressMigrationAndUpdate(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)

	_, err = db.ExecContext(ctx, `
		CREATE TABLE genres (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE
		);
		CREATE TABLE books (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			openlibrary_edition_id TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL,
			author TEXT NOT NULL,
			cover_url TEXT NOT NULL DEFAULT '',
			is_next_pal INTEGER NOT NULL DEFAULT 0,
			reading_status TEXT NOT NULL DEFAULT 'to_read',
			genre_id INTEGER REFERENCES genres(id) ON DELETE SET NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatal(err)
	}

	library := New(db)
	if err := library.Init(ctx); err != nil {
		t.Fatal(err)
	}

	created, err := library.CreateBook(ctx, models.Book{
		OpenLibraryEditionID: "OL1M",
		Title:                "Livre test",
	})
	if err != nil {
		t.Fatal(err)
	}

	totalPages := 300
	updated, err := library.UpdateBookProgress(ctx, created.ID, 42, &totalPages)
	if err != nil {
		t.Fatal(err)
	}
	if updated.PagesRead != 42 || updated.TotalPages == nil || updated.TotalPagesValue() != 300 {
		t.Fatalf("unexpected progress: %+v", updated)
	}

	if _, err := library.UpdateBookProgress(ctx, created.ID, 301, &totalPages); !errors.Is(err, ErrInvalidReadingProgress) {
		t.Fatalf("expected invalid progress, got %v", err)
	}

	books, err := library.ListBooks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(books) != 1 || books[0].PagesRead != 42 || books[0].TotalPagesValue() != 300 {
		t.Fatalf("unexpected listed books: %+v", books)
	}
}

func TestSearchBooksByTitle(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)

	library := New(db)
	if err := library.Init(ctx); err != nil {
		t.Fatal(err)
	}

	_, err = library.CreateBook(ctx, models.Book{
		OpenLibraryEditionID: "OL1M",
		Title:                "Le Petit Prince",
		Author:               "Antoine de Saint-Exupéry",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = library.CreateBook(ctx, models.Book{
		OpenLibraryEditionID: "OL2M",
		Title:                "Germinal",
		Author:               "Émile Zola",
	})
	if err != nil {
		t.Fatal(err)
	}

	books, err := library.SearchBooksByTitle(ctx, "petit")
	if err != nil {
		t.Fatal(err)
	}
	if len(books) != 1 || books[0].Title != "Le Petit Prince" {
		t.Fatalf("unexpected title search results: %+v", books)
	}

	books, err = library.SearchBooksByTitle(ctx, "Zola")
	if err != nil {
		t.Fatal(err)
	}
	if len(books) != 0 {
		t.Fatalf("author matched title search: %+v", books)
	}
}
