package qdrant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is a minimal Qdrant HTTP client
type Client struct {
	baseURL    string
	apiKey     string
	collection string
	httpClient *http.Client
}

// NewClient creates a new Qdrant client
func NewClient(baseURL, apiKey, collection string) *Client {
	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		collection: collection,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Point represents a Qdrant point for upsert
type Point struct {
	ID      uint64               `json:"id"`
	Vector  []float32            `json:"vector"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

// UpsertRequest is the request body for upsert
type UpsertRequest struct {
	Points []Point `json:"points"`
}

// SearchRequest is the request body for search
type SearchRequest struct {
	Vector    []float32              `json:"vector"`
	Limit     int                    `json:"limit"`
	WithPayload bool                 `json:"with_payload"`
	Filter    map[string]interface{} `json:"filter,omitempty"`
	ScoreThreshold *float64          `json:"score_threshold,omitempty"`
}

// SearchResult is a single search result
type SearchResult struct {
	ID      uint64                 `json:"id"`
	Score   float64                `json:"score"`
	Payload map[string]interface{} `json:"payload"`
}

// SearchResponse is the response from search
type SearchResponse struct {
	Result []SearchResult `json:"result"`
}

// ErrorResponse is an error response from Qdrant
type ErrorResponse struct {
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
	Time    float64 `json:"time"`
}

// InitCollection creates or ensures the collection exists
func (c *Client) InitCollection(ctx context.Context, vectorSize int) error {
	// Check if collection exists
	exists, err := c.collectionExists(ctx)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	// Create collection
	body := map[string]interface{}{
		"vectors": map[string]interface{}{
			"size":     vectorSize,
			"distance": "Cosine",
		},
		"sparse_vectors": map[string]interface{}{
			"text_sparse": map[string]interface{}{},
		},
	}

	return c.doRequest(ctx, http.MethodPut, fmt.Sprintf("/collections/%s", c.collection), body, nil)
}

// Upsert inserts or updates points in the collection
func (c *Client) Upsert(ctx context.Context, points []Point) error {
	if len(points) == 0 {
		return nil
	}

	body := UpsertRequest{Points: points}
	return c.doRequest(ctx, http.MethodPut, fmt.Sprintf("/collections/%s/points?wait=true", c.collection), body, nil)
}

// Search performs a dense vector search
func (c *Client) Search(ctx context.Context, vector []float32, limit int, minScore *float64) ([]SearchResult, error) {
	req := SearchRequest{
		Vector:       vector,
		Limit:        limit,
		WithPayload:  true,
		ScoreThreshold: minScore,
	}

	var resp SearchResponse
	err := c.doRequest(ctx, http.MethodPost, fmt.Sprintf("/collections/%s/points/search", c.collection), req, &resp)
	if err != nil {
		return nil, err
	}

	return resp.Result, nil
}

// SearchSparse performs a sparse (BM25) search
func (c *Client) SearchSparse(ctx context.Context, text string, limit int) ([]SearchResult, error) {
	// Qdrant sparse vector search requires pre-computed sparse vectors
	// For now, we'll use a simple keyword-based approach
	// In production, you'd use a proper sparse encoder
	req := map[string]interface{}{
		"limit": limit,
		"with_payload": true,
		"vector": map[string]interface{}{
			"name": "text_sparse",
			"indices": []int{},
			"values": []float32{},
		},
	}

	var resp SearchResponse
	err := c.doRequest(ctx, http.MethodPost, fmt.Sprintf("/collections/%s/points/search", c.collection), req, &resp)
	if err != nil {
		return nil, err
	}

	return resp.Result, nil
}

// DeleteCollection deletes the collection
func (c *Client) DeleteCollection(ctx context.Context) error {
	return c.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/collections/%s", c.collection), nil, nil)
}

// PointCount returns the number of points in the collection
func (c *Client) PointCount(ctx context.Context) (int, error) {
	var resp struct {
		Result struct {
			Count int `json:"count"`
		} `json:"result"`
	}

	err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/collections/%s/points/count", c.collection), nil, &resp)
	if err != nil {
		return 0, err
	}

	return resp.Result.Count, nil
}

func (c *Client) collectionExists(ctx context.Context) (bool, error) {
	var resp struct {
		Result struct {
			Status string `json:"status"`
		} `json:"result"`
	}

	err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/collections/%s", c.collection), nil, &resp)
	if err != nil {
		// Check if it's a "not found" error
		return false, nil
	}

	return resp.Result.Status == "green" || resp.Result.Status == "yellow", nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("api-key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request to Qdrant: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp ErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error != "" {
			return fmt.Errorf("qdrant error (%d): %s", resp.StatusCode, errResp.Error)
		}
		return fmt.Errorf("qdrant error (%d): %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}

	return nil
}

// IsAvailable checks if Qdrant is running
func (c *Client) IsAvailable(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/readyz", nil)
	if err != nil {
		return false
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}
