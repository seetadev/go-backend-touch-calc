package meshkit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultBaseURL = "http://localhost:3001"
	defaultTimeout = 30 * time.Second
)

// Client is a thin HTTP client for the meshkit-service sidecar.
type Client struct {
	baseURL string
	http    *http.Client
}

// NewClient returns a Client for the given base URL. An empty baseURL falls
// back to the MESHKIT_SERVICE_URL environment variable, then to
// http://localhost:3001.
func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = os.Getenv("MESHKIT_SERVICE_URL")
	}
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: defaultTimeout},
	}
}

// UploadFile stores content under the given path and returns the resulting CID.
func (c *Client) UploadFile(ctx context.Context, path, content string) (string, error) {
	body, err := json.Marshal(map[string]string{"path": path, "content": content})
	if err != nil {
		return "", fmt.Errorf("meshkit: marshal upload request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/files/put", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("meshkit: build upload request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("meshkit: upload request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("meshkit: upload failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	var result struct {
		CID  string `json:"cid"`
		Path string `json:"path"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("meshkit: decode upload response: %w", err)
	}
	if result.CID == "" {
		return "", fmt.Errorf("meshkit: upload response missing cid")
	}
	return result.CID, nil
}

// GetFile retrieves the content stored under the given CID.
func (c *Client) GetFile(ctx context.Context, cid string) ([]byte, error) {
	endpoint := c.baseURL + "/files/" + url.PathEscape(cid)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("meshkit: build get request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("meshkit: get request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("meshkit: get failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("meshkit: read get response: %w", err)
	}
	return data, nil
}

// HealthCheck verifies the sidecar is reachable.
func (c *Client) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("meshkit: build health request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("meshkit: health request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("meshkit: health check failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return nil
}
