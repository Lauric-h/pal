package googlebooks

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

const endpoint = "https://www.googleapis.com/books/v1/volumes"

type Client struct {
	httpClient *http.Client
	apiKey     string
}

func New(apiKey string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		apiKey:     strings.TrimSpace(apiKey),
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
	params.Set("maxResults", "10")
	if c.apiKey != "" {
		params.Set("key", c.apiKey)
	}
	requestURL.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("google books: unexpected HTTP status %s", resp.Status)
	}

	var payload apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("google books: decode response: %w", err)
	}

	books := make([]models.Book, 0, len(payload.Items))
	for _, item := range payload.Items {
		id := strings.TrimSpace(item.ID)
		title := strings.TrimSpace(item.VolumeInfo.Title)
		if id == "" || title == "" {
			continue
		}
		coverURL := strings.Replace(item.VolumeInfo.ImageLinks.Thumbnail, "http://", "https://", 1)
		var totalPages *int
		if item.VolumeInfo.PageCount > 0 {
			pageCount := item.VolumeInfo.PageCount
			totalPages = &pageCount
		}
		books = append(books, models.Book{
			OpenLibraryEditionID: "google:" + id,
			Title:                title,
			Author:               strings.Join(item.VolumeInfo.Authors, ", "),
			CoverURL:             coverURL,
			TotalPages:           totalPages,
		})
	}
	return books, nil
}

type apiResponse struct {
	Items []struct {
		ID         string `json:"id"`
		VolumeInfo struct {
			Title      string   `json:"title"`
			Authors    []string `json:"authors"`
			PageCount  int      `json:"pageCount"`
			ImageLinks struct {
				Thumbnail string `json:"thumbnail"`
			} `json:"imageLinks"`
		} `json:"volumeInfo"`
	} `json:"items"`
}
