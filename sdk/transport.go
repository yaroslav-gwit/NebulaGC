package sdk

import (
	"context"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"time"
)

// doRequestWithRetry performs an HTTP request with exponential backoff retry logic.
// It will retry on network errors and 5xx server errors.
func (c *Client) doRequestWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := 0; attempt <= c.RetryAttempts; attempt++ {
		// Perform the request
		resp, err = c.HTTPClient.Do(req.WithContext(ctx))

		// If successful (2xx or 4xx), return immediately
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}

		// If this was the last attempt, return the error
		if attempt == c.RetryAttempts {
			if resp != nil {
				resp.Body.Close()
			}
			break
		}

		// Close response body if present
		if resp != nil {
			resp.Body.Close()
		}

		// Calculate backoff duration with exponential backoff and jitter
		backoff := c.calculateBackoff(attempt)

		// Wait for backoff duration or until context is cancelled
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
			// Continue to next attempt
		}

		// If the request had a body, we need to recreate it for the retry
		// (The original body reader may have been consumed)
		// This is handled by the caller through GetBody
	}

	// Return the last error
	if err != nil {
		return nil, fmt.Errorf("request failed after %d attempts: %w", c.RetryAttempts+1, err)
	}

	// Server error after all retries
	if resp != nil && resp.StatusCode >= 500 {
		return resp, fmt.Errorf("%w: status code %d", ErrServerError, resp.StatusCode)
	}

	return resp, err
}

// calculateBackoff calculates the backoff duration for a retry attempt.
// It uses exponential backoff with jitter to avoid thundering herd.
func (c *Client) calculateBackoff(attempt int) time.Duration {
	// Exponential backoff: min * (2 ^ attempt)
	backoff := float64(c.RetryWaitMin) * math.Pow(2, float64(attempt))

	// Cap at maximum wait time
	if backoff > float64(c.RetryWaitMax) {
		backoff = float64(c.RetryWaitMax)
	}

	// Add jitter (random value between 0 and backoff)
	jitter := rand.Float64() * backoff

	return time.Duration(jitter)
}

// drainAndCloseBody reads and closes the response body to ensure connection reuse.
func drainAndCloseBody(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}
