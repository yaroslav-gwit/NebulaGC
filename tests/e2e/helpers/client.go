package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClient wraps httptest for easier API testing.
type TestClient struct {
	Server  *httptest.Server
	BaseURL string
	t       *testing.T
}

// NewTestClient creates a new test HTTP client.
func NewTestClient(t *testing.T, handler http.Handler) *TestClient {
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return &TestClient{
		Server:  server,
		BaseURL: server.URL,
		t:       t,
	}
}

// Request represents an HTTP request configuration.
type Request struct {
	Method  string
	Path    string
	Body    interface{}
	Headers map[string]string
}

// Response represents an HTTP response with helpers.
type Response struct {
	*http.Response
	Body []byte
	t    *testing.T
}

// Do executes an HTTP request and returns a response.
func (c *TestClient) Do(req Request) *Response {
	c.t.Helper()

	// Build URL
	url := c.BaseURL + req.Path

	// Prepare body
	var bodyReader io.Reader
	if req.Body != nil {
		bodyBytes, err := json.Marshal(req.Body)
		require.NoError(c.t, err, "failed to marshal request body")
		bodyReader = bytes.NewReader(bodyBytes)
	}

	// Create request
	httpReq, err := http.NewRequest(req.Method, url, bodyReader)
	require.NoError(c.t, err, "failed to create request")

	// Set headers
	if req.Body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// Execute request
	httpResp, err := http.DefaultClient.Do(httpReq)
	require.NoError(c.t, err, "request failed")

	// Read body
	bodyBytes, err := io.ReadAll(httpResp.Body)
	require.NoError(c.t, err, "failed to read response body")
	httpResp.Body.Close()

	return &Response{
		Response: httpResp,
		Body:     bodyBytes,
		t:        c.t,
	}
}

// GET executes a GET request.
func (c *TestClient) GET(path string, headers ...map[string]string) *Response {
	req := Request{
		Method: http.MethodGet,
		Path:   path,
	}
	if len(headers) > 0 {
		req.Headers = headers[0]
	}
	return c.Do(req)
}

// POST executes a POST request.
func (c *TestClient) POST(path string, body interface{}, headers ...map[string]string) *Response {
	req := Request{
		Method: http.MethodPost,
		Path:   path,
		Body:   body,
	}
	if len(headers) > 0 {
		req.Headers = headers[0]
	}
	return c.Do(req)
}

// PATCH executes a PATCH request.
func (c *TestClient) PATCH(path string, body interface{}, headers ...map[string]string) *Response {
	req := Request{
		Method: http.MethodPatch,
		Path:   path,
		Body:   body,
	}
	if len(headers) > 0 {
		req.Headers = headers[0]
	}
	return c.Do(req)
}

// DELETE executes a DELETE request.
func (c *TestClient) DELETE(path string, headers ...map[string]string) *Response {
	req := Request{
		Method: http.MethodDelete,
		Path:   path,
	}
	if len(headers) > 0 {
		req.Headers = headers[0]
	}
	return c.Do(req)
}

// AssertStatus asserts the response status code.
func (r *Response) AssertStatus(expected int) *Response {
	r.t.Helper()
	assert.Equal(r.t, expected, r.StatusCode,
		"unexpected status code\nBody: %s", string(r.Body))
	return r
}

// RequireStatus requires the response status code.
func (r *Response) RequireStatus(expected int) *Response {
	r.t.Helper()
	require.Equal(r.t, expected, r.StatusCode,
		"unexpected status code\nBody: %s", string(r.Body))
	return r
}

// AssertJSON unmarshals JSON and asserts no error.
func (r *Response) AssertJSON(v interface{}) *Response {
	r.t.Helper()
	err := json.Unmarshal(r.Body, v)
	assert.NoError(r.t, err, "failed to unmarshal JSON: %s", string(r.Body))
	return r
}

// RequireJSON unmarshals JSON and requires no error.
func (r *Response) RequireJSON(v interface{}) *Response {
	r.t.Helper()
	err := json.Unmarshal(r.Body, v)
	require.NoError(r.t, err, "failed to unmarshal JSON: %s", string(r.Body))
	return r
}

// AssertError asserts the response contains an error with specific code.
func (r *Response) AssertError(errorCode string) *Response {
	r.t.Helper()

	var errResp struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}

	err := json.Unmarshal(r.Body, &errResp)
	require.NoError(r.t, err, "failed to unmarshal error response")

	assert.Equal(r.t, errorCode, errResp.Error,
		"unexpected error code\nMessage: %s", errResp.Message)

	return r
}

// GetHeader returns a response header value.
func (r *Response) GetHeader(key string) string {
	return r.Header.Get(key)
}

// BodyString returns the response body as a string.
func (r *Response) BodyString() string {
	return string(r.Body)
}

// PrintBody prints the response body (for debugging).
func (r *Response) PrintBody() *Response {
	fmt.Printf("Response Body:\n%s\n", string(r.Body))
	return r
}

// AssertHeader asserts a response header value.
func (r *Response) AssertHeader(key, expected string) *Response {
	r.t.Helper()
	actual := r.Header.Get(key)
	assert.Equal(r.t, expected, actual, "unexpected header value for %s", key)
	return r
}

// AssertContains asserts the body contains a substring.
func (r *Response) AssertContains(substr string) *Response {
	r.t.Helper()
	assert.Contains(r.t, string(r.Body), substr, "response body should contain substring")
	return r
}
