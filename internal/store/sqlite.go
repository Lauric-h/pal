package store

import (
	"context"
	"database/sql"
	"errors"

	"pal/internal/models"

	_ "modernc.org/sqlite"
)

var (
	ErrBookExists             = errors.New("book already exists")
	ErrInvalidReadingStatus   = errors.New("invalid reading status")
	ErrInvalidReadingProgress = errors.New("invalid reading progress")
	ErrGenreNotFound          = errors.New("genre not found")
)

type bookColumn struct {
	name       string
	definition string
}

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Init(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS genres (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE
		)
	`); err != nil {
		return err
	}
	for _, name := range models.DefaultGenres {
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO genres (name) VALUES (?) ON CONFLICT(name) DO NOTHING
		`, name); err != nil {
			return err
		}
	}

	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS books (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			openlibrary_edition_id TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL,
			author TEXT NOT NULL,
			cover_url TEXT NOT NULL DEFAULT '',
			is_next_pal INTEGER NOT NULL DEFAULT 0,
			reading_status TEXT NOT NULL DEFAULT 'to_read',
			genre_id INTEGER REFERENCES genres(id) ON DELETE SET NULL,
			total_pages INTEGER CHECK (total_pages IS NULL OR total_pages > 0),
			pages_read INTEGER NOT NULL DEFAULT 0 CHECK (pages_read >= 0),
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return err
	}

	if err := s.ensureBookColumns(ctx); err != nil {
		return err
	}

	return nil
}

func (s *Store) ensureBookColumns(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(books)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, primaryKey int
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return err
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	for _, column := range []bookColumn{
		{name: "total_pages", definition: "INTEGER CHECK (total_pages IS NULL OR total_pages > 0)"},
		{name: "pages_read", definition: "INTEGER NOT NULL DEFAULT 0 CHECK (pages_read >= 0)"},
	} {
		if columns[column.name] {
			continue
		}
		if _, err := s.db.ExecContext(ctx, "ALTER TABLE books ADD COLUMN "+column.name+" "+column.definition); err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) CreateBook(ctx context.Context, book models.Book) (models.Book, error) {
	const query = `
		INSERT INTO books (openlibrary_edition_id, title, author, cover_url, total_pages)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(openlibrary_edition_id) DO NOTHING
		RETURNING id, openlibrary_edition_id, title, author, cover_url, is_next_pal, reading_status, genre_id, total_pages, pages_read, created_at
	`

	var totalPages any
	if book.TotalPages != nil {
		totalPages = *book.TotalPages
	}

	var created models.Book
	var genreID, storedTotalPages sql.NullInt64
	err := s.db.QueryRowContext(ctx, query,
		book.OpenLibraryEditionID,
		book.Title,
		book.Author,
		book.CoverURL,
		totalPages,
	).Scan(
		&created.ID,
		&created.OpenLibraryEditionID,
		&created.Title,
		&created.Author,
		&created.CoverURL,
		&created.IsNextPal,
		&created.ReadingStatus,
		&genreID,
		&storedTotalPages,
		&created.PagesRead,
		&created.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Book{}, ErrBookExists
	}
	created.GenreID = nullableID(genreID)
	created.TotalPages = nullableInt(storedTotalPages)
	return created, err
}

func (s *Store) ListBooks(ctx context.Context) ([]models.Book, error) {
	return s.ListBooksFiltered(ctx, nil, "")
}

func (s *Store) ListBooksFiltered(ctx context.Context, genreID *int64, status models.ReadingStatus) ([]models.Book, error) {
	if status != "" && !models.IsValidReadingStatus(status) {
		return nil, ErrInvalidReadingStatus
	}

	var genreFilter int64
	if genreID != nil {
		genreFilter = *genreID
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT b.id, b.openlibrary_edition_id, b.title, b.author, b.cover_url,
			b.is_next_pal, b.reading_status, g.id, g.name, b.total_pages, b.pages_read, b.created_at
		FROM books b
		LEFT JOIN genres g ON g.id = b.genre_id
		WHERE (? = 0 OR b.genre_id = ?)
			AND (? = '' OR b.reading_status = ?)
		ORDER BY LOWER(b.title) COLLATE NOCASE ASC,
			b.id ASC
	`, genreFilter, genreFilter, status, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	books := make([]models.Book, 0)
	for rows.Next() {
		book, err := scanBook(rows)
		if err != nil {
			return nil, err
		}
		books = append(books, book)
	}
	return books, rows.Err()
}

func (s *Store) ListGenres(ctx context.Context) ([]models.Genre, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT g.id, g.name, COUNT(b.id)
		FROM genres g
		LEFT JOIN books b ON b.genre_id = g.id
		GROUP BY g.id, g.name
		ORDER BY g.name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	genres := make([]models.Genre, 0, len(models.DefaultGenres))
	for rows.Next() {
		var genre models.Genre
		if err := rows.Scan(&genre.ID, &genre.Name, &genre.Count); err != nil {
			return nil, err
		}
		genres = append(genres, genre)
	}
	return genres, rows.Err()
}

func (s *Store) ToggleNextPal(ctx context.Context, id int64) (models.Book, error) {
	const query = `
		UPDATE books
		SET is_next_pal = CASE is_next_pal WHEN 1 THEN 0 ELSE 1 END
		WHERE id = ?
		RETURNING id, openlibrary_edition_id, title, author, cover_url, is_next_pal, reading_status, genre_id, total_pages, pages_read, created_at
	`

	var book models.Book
	var genreID, totalPages sql.NullInt64
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&book.ID,
		&book.OpenLibraryEditionID,
		&book.Title,
		&book.Author,
		&book.CoverURL,
		&book.IsNextPal,
		&book.ReadingStatus,
		&genreID,
		&totalPages,
		&book.PagesRead,
		&book.CreatedAt,
	)
	book.GenreID = nullableID(genreID)
	book.TotalPages = nullableInt(totalPages)
	return book, err
}

func (s *Store) UpdateReadingStatus(ctx context.Context, id int64, status models.ReadingStatus) (models.Book, error) {
	if !models.IsValidReadingStatus(status) {
		return models.Book{}, ErrInvalidReadingStatus
	}

	const query = `
		UPDATE books
		SET reading_status = ?,
			is_next_pal = CASE WHEN ? = 'finished' THEN 0 ELSE is_next_pal END
		WHERE id = ?
		RETURNING id, openlibrary_edition_id, title, author, cover_url, is_next_pal, reading_status, genre_id, total_pages, pages_read, created_at
	`

	var book models.Book
	var genreID, totalPages sql.NullInt64
	err := s.db.QueryRowContext(ctx, query, status, status, id).Scan(
		&book.ID,
		&book.OpenLibraryEditionID,
		&book.Title,
		&book.Author,
		&book.CoverURL,
		&book.IsNextPal,
		&book.ReadingStatus,
		&genreID,
		&totalPages,
		&book.PagesRead,
		&book.CreatedAt,
	)
	book.GenreID = nullableID(genreID)
	book.TotalPages = nullableInt(totalPages)
	return book, err
}
func (s *Store) UpdateBookProgress(ctx context.Context, id int64, pagesRead int, totalPages *int) (models.Book, error) {
	if pagesRead < 0 || totalPages != nil && (*totalPages <= 0 || pagesRead > *totalPages) {
		return models.Book{}, ErrInvalidReadingProgress
	}

	var totalPagesValue any
	if totalPages != nil {
		totalPagesValue = *totalPages
	}

	const query = `
		UPDATE books
		SET pages_read = ?, total_pages = ?
		WHERE id = ?
		RETURNING id, openlibrary_edition_id, title, author, cover_url, is_next_pal, reading_status, genre_id, total_pages, pages_read, created_at
	`

	var book models.Book
	var genreID, storedTotalPages sql.NullInt64
	err := s.db.QueryRowContext(ctx, query, pagesRead, totalPagesValue, id).Scan(
		&book.ID,
		&book.OpenLibraryEditionID,
		&book.Title,
		&book.Author,
		&book.CoverURL,
		&book.IsNextPal,
		&book.ReadingStatus,
		&genreID,
		&storedTotalPages,
		&book.PagesRead,
		&book.CreatedAt,
	)
	book.GenreID = nullableID(genreID)
	book.TotalPages = nullableInt(storedTotalPages)
	return book, err
}

func (s *Store) UpdateBookGenre(ctx context.Context, id int64, genreID *int64) error {
	var value any
	if genreID != nil {
		var exists int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM genres WHERE id = ?`, *genreID).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return ErrGenreNotFound
		}
		value = *genreID
	}

	result, err := s.db.ExecContext(ctx, `UPDATE books SET genre_id = ? WHERE id = ?`, value, id)
	if err != nil {
		return err
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if updated == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) DeleteBook(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM books WHERE id = ?`, id)
	if err != nil {
		return err
	}

	deleted, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if deleted == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func scanBook(scanner interface{ Scan(...any) error }) (models.Book, error) {
	var book models.Book
	var genreID, totalPages sql.NullInt64
	var genreName sql.NullString
	err := scanner.Scan(
		&book.ID,
		&book.OpenLibraryEditionID,
		&book.Title,
		&book.Author,
		&book.CoverURL,
		&book.IsNextPal,
		&book.ReadingStatus,
		&genreID,
		&genreName,
		&totalPages,
		&book.PagesRead,
		&book.CreatedAt,
	)
	book.GenreID = nullableID(genreID)
	book.TotalPages = nullableInt(totalPages)
	if genreName.Valid {
		book.GenreName = genreName.String
	}
	return book, err
}

func nullableInt(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	result := int(value.Int64)
	return &result
}

func nullableID(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	id := value.Int64
	return &id
}
