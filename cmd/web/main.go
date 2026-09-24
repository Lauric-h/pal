package main

import (
	"context"
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"pal/internal/googlebooks"
	"pal/internal/handlers"
	"pal/internal/openlibrary"
	"pal/internal/store"

	"github.com/joho/godotenv"
)

func main() {
	ctx := context.Background()
	if err := godotenv.Load(); err != nil {
		log.Println("pas de fichier .env trouvé, on continue avec l'environnement système")
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/library.db"
	}
	if dir := filepath.Dir(dbPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatal(err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		log.Fatal(err)
	}

	books := store.New(db)
	if err := books.Init(ctx); err != nil {
		log.Fatal(err)
	}

	templates, err := template.ParseGlob("ui/html/*.tmpl")
	if err != nil {
		log.Fatal(err)
	}

	app := &handlers.Application{
		Store:       books,
		OpenLibrary: openlibrary.New(os.Getenv("OPENLIBRARY_USER_AGENT")),
		GoogleBooks: googlebooks.New(os.Getenv("GOOGLE_BOOKS_API_KEY")),
		Templates:   templates,
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	server := &http.Server{
		Addr:    ":" + port,
		Handler: app.Routes(),
	}

	log.Printf("listening on http://localhost:%s", port)
	log.Fatal(server.ListenAndServe())
}
