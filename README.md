# PAL

**PAL** stands for **_Pile à Lire_**, French for “to-read pile”. It is a self-hosted personal library for keeping that pile useful instead of letting it disappear into a spreadsheet.

PAL helps you search for books, add them to your library, organize them by genre and reading status, and maintain a focused **Pile à Lire**.

## Features

- Search through Open Library or Google Books
- Add books with title, author, cover
- Add genres and filter the library by genre or status
- Mark books as part of the PAL
- Persist data in SQLite
- Dark and light themes
- Responsive server-rendered UI with HTMX interactions
- Docker image and Docker Compose setup

## Self-host with Docker Compose

Requirements:

- Docker
- Docker Compose

Clone the repository, create the persistent data directory, and start PAL:

```bash
git clone https://github.com/Lauric-h/pal.git
cd pal
mkdir -p data
docker compose up -d
```

Open [http://localhost:8080](http://localhost:8080).

The SQLite database is stored at `./data/library.db` through the `/data` volume. Stopping or recreating the container does not remove your library.

Useful commands:

```bash
# Follow application logs
docker compose logs -f pal

# Stop PAL
docker compose down

# Update to the latest published image
docker compose pull
docker compose up -d
```

### Optional Google Books API key

Open Library works without a key. To enable a Google Books API key, export it before starting Compose:

```bash
export GOOGLE_BOOKS_API_KEY="your-api-key"
docker compose up -d
```

The Compose file passes the key to the container without storing it in the repository.

## Run locally with Go

Requirements:

- Go 1.26+

Start the server:

```bash
go run ./cmd/web
```

Open [http://localhost:8080](http://localhost:8080).

Configuration is provided through environment variables:

| Variable                 | Default                      | Purpose                         |
| ------------------------ | ---------------------------- | ------------------------------- |
| `PORT`                   | `8080`                       | HTTP server port                |
| `DB_PATH`                | `library.db`                 | SQLite database path            |
| `OPENLIBRARY_USER_AGENT` | `pal/1.0 (personal library)` | User-Agent sent to Open Library |
| `GOOGLE_BOOKS_API_KEY`   | empty                        | Optional Google Books API key   |

Example:

```bash
DB_PATH=./data/library.db PORT=8080 go run ./cmd/web
```

Run the Go checks:

```bash
go test ./...
```

## Project structure

```text
cmd/web/                 Application entrypoint
internal/handlers/       HTTP routes and page composition
internal/models/         Domain models and reading statuses
internal/store/          SQLite schema and persistence
internal/openlibrary/    Open Library client
internal/googlebooks/    Google Books client
ui/html/                 Server-rendered HTML templates
Dockerfile               Multi-stage production image
docker-compose.yml       Self-hosting configuration
```
