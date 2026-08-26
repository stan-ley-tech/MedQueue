//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// apiClient is a minimal HTTP client for driving the e2e workflow tests
// against a live httptest.Server without pulling in a full SDK.
type apiClient struct {
	t       *testing.T
	baseURL string
	token   string
}

func newAPIClient(t *testing.T, baseURL string) *apiClient {
	return &apiClient{t: t, baseURL: baseURL}
}

func (c *apiClient) withToken(token string) *apiClient {
	return &apiClient{t: c.t, baseURL: c.baseURL, token: token}
}

func (c *apiClient) do(method, path string, body any, out any) (*http.Response, []byte) {
	c.t.Helper()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			c.t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reader)
	if err != nil {
		c.t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.t.Fatalf("read response body: %v", err)
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			c.t.Fatalf("%s %s: unmarshal response: %v\nbody: %s", method, path, err, respBody)
		}
	}
	return resp, respBody
}

func (c *apiClient) requireStatus(resp *http.Response, body []byte, want int) {
	c.t.Helper()
	if resp.StatusCode != want {
		c.t.Fatalf("%s %s: status = %d, want %d\nbody: %s", resp.Request.Method, resp.Request.URL.Path, resp.StatusCode, want, body)
	}
}

// idempotentClient wraps apiClient to attach a fixed Idempotency-Key to
// every request, for tests that specifically exercise retry-safety.
type idempotentClient struct {
	*apiClient
	key string
}

func (c *idempotentClient) do(method, path string, body any, out any) (*http.Response, []byte) {
	c.t.Helper()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			c.t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reader)
	if err != nil {
		c.t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", c.key)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.t.Fatalf("read response body: %v", err)
	}
	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			c.t.Fatalf("%s %s: unmarshal response: %v\nbody: %s", method, path, err, respBody)
		}
	}
	return resp, respBody
}
