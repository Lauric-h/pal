package hardcover

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestSearch(t *testing.T) {
	client := New("token")
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("missing authorization header")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), "Dune") {
			t.Fatalf("query missing search term: %s", body)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"data":{"search":{"ids":[123],"results":{"hits":[{"document":{"id":"123","title":"Dune","author_names":["Frank Herbert"],"pages":412,"image":{"url":"https://example.com/dune.jpg"}}}]}}}}`)),
			Header:     make(http.Header),
		}, nil
	})}

	books, err := client.Search(context.Background(), "Dune")
	if err != nil {
		t.Fatal(err)
	}
	if len(books) != 1 || books[0].OpenLibraryEditionID != "hardcover:123" || books[0].Title != "Dune" || books[0].Author != "Frank Herbert" || books[0].CoverURL != "https://example.com/dune.jpg" || books[0].TotalPagesValue() != 412 {
		t.Fatalf("unexpected books: %+v", books)
	}
}
