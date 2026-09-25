package openlibrary

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"pal/internal/models"
)

const endpoint = "https://openlibrary.org/search.json"

type Client struct {
	httpClient *http.Client
	userAgent  string
}

func New(userAgent string) *Client {
	if strings.TrimSpace(userAgent) == "" {
		userAgent = "pal/1.0 (personal library)"
	}
	return &Client{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		userAgent:  userAgent,
	}
}

func (c *Client) Search(ctx context.Context, query string) ([]models.Book, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	requestURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	params := requestURL.Query()
	params.Set("q", query)
	params.Set("fields", "key,title,author_name,cover_i,edition_key")
	params.Set("limit", "10")
	params.Set("lang", "fr")
	requestURL.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("open library: unexpected HTTP status %s", resp.Status)
	}

	var payload apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("open library: decode response: %w", err)
	}

	books := make([]models.Book, 0, len(payload.Docs))
	for _, doc := range payload.Docs {
		title := strings.TrimSpace(doc.Title)
		id := editionID(doc.EditionKeys, doc.Key)
		if id == "" || title == "" {
			continue
		}

		coverURL := ""
		if doc.CoverID > 0 {
			coverURL = fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-M.jpg?default=false", doc.CoverID)
		}
		books = append(books, models.Book{
			OpenLibraryEditionID: id,
			Title:                title,
			Author:               strings.Join(doc.Authors, ", "),
			CoverURL:             coverURL,
		})
	}
	return books, nil
}

func editionID(keys []string, workKey string) string {
	for _, key := range keys {
		if key = strings.TrimSpace(key); key != "" {
			return key
		}
	}
	workKey = strings.TrimSpace(workKey)
	workKey = strings.TrimPrefix(workKey, "/works/")
	return strings.TrimPrefix(workKey, "/books/")
}

type apiResponse struct {
	Docs []struct {
		Key         string   `json:"key"`
		Title       string   `json:"title"`
		Authors     []string `json:"author_name"`
		CoverID     int      `json:"cover_i"`
		EditionKeys []string `json:"edition_key"`
	} `json:"docs"`
}
