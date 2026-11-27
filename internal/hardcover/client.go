package hardcover

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"pal/internal/models"
)

const endpoint = "https://api.hardcover.app/v1/graphql"

type Client struct {
	httpClient *http.Client
	token      string
}

func New(token string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		token:      strings.TrimSpace(token),
	}
}

func (c *Client) Search(ctx context.Context, query string) ([]models.Book, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if c.token == "" {
		return nil, fmt.Errorf("hardcover: API key is not configured")
	}

	body, err := json.Marshal(map[string]string{
		"query": fmt.Sprintf(`query { search(query: %s, query_type: "Book", per_page: 10, page: 1) { ids results } }`, strconv.Quote(query)),
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("User-Agent", "PAL/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("hardcover: unexpected HTTP status %s", resp.Status)
	}

	var payload apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("hardcover: decode response: %w", err)
	}
	if len(payload.Errors) > 0 {
		return nil, fmt.Errorf("hardcover: %s", payload.Errors[0].Message)
	}

	books := make([]models.Book, 0, len(payload.Data.Search.Results.Hits))
	for index, hit := range payload.Data.Search.Results.Hits {
		result := hit.Document
		id := rawInt64(result["id"])
		if id == 0 && index < len(payload.Data.Search.IDs) {
			id = payload.Data.Search.IDs[index]
		}
		title := strings.TrimSpace(rawString(result["title"]))
		if id == 0 || title == "" {
			continue
		}
		books = append(books, models.Book{
			OpenLibraryEditionID: fmt.Sprintf("hardcover:%d", id),
			Title:                title,
			Author:               strings.Join(rawStrings(result["author_names"]), ", "),
			CoverURL:             rawImageURL(result["image"]),
			TotalPages:           positiveIntPointer(rawInt(result["pages"])),
		})
	}
	return books, nil
}

type apiResponse struct {
	Data struct {
		Search struct {
			IDs     []int64 `json:"ids"`
			Results struct {
				Hits []struct {
					Document map[string]json.RawMessage `json:"document"`
				} `json:"hits"`
			} `json:"results"`
		} `json:"search"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func rawString(value json.RawMessage) string {
	var result string
	if err := json.Unmarshal(value, &result); err != nil {
		return ""
	}
	return result
}

func rawInt(value json.RawMessage) int {
	var result int
	if err := json.Unmarshal(value, &result); err != nil {
		return 0
	}
	return result
}
func rawInt64(value json.RawMessage) int64 {
	var result int64
	if err := json.Unmarshal(value, &result); err == nil {
		return result
	}
	var text string
	if err := json.Unmarshal(value, &text); err != nil {
		return 0
	}
	result, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return 0
	}
	return result
}

func rawStrings(value json.RawMessage) []string {
	var result []string
	if err := json.Unmarshal(value, &result); err == nil {
		return result
	}
	if value := rawString(value); value != "" {
		return []string{value}
	}
	return nil
}

func rawImageURL(value json.RawMessage) string {
	if url := rawString(value); url != "" {
		return url
	}
	var image struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(value, &image); err != nil {
		return ""
	}
	return image.URL
}

func positiveIntPointer(value int) *int {
	if value <= 0 {
		return nil
	}
	return &value
}
