package orca

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClientFromEnv(t *testing.T) {
	// Given: env vars are set
	t.Setenv("ORCA_API_KEY", "env-key")
	t.Setenv("ORCA_API_URL", "http://localhost:9090")

	// When
	c := NewClient()

	// Then
	if c.apiKey != "env-key" {
		t.Errorf("expected apiKey=env-key, got %s", c.apiKey)
	}
	if c.baseURL != "http://localhost:9090" {
		t.Errorf("expected baseURL=http://localhost:9090, got %s", c.baseURL)
	}
}

func TestNewClientOptionsOverrideEnv(t *testing.T) {
	// Given: env vars and explicit options
	t.Setenv("ORCA_API_KEY", "env-key")

	// When
	c := NewClient(WithAPIKey("opt-key"), WithBaseURL("http://example.com"))

	// Then: options win
	if c.apiKey != "opt-key" {
		t.Errorf("expected apiKey=opt-key, got %s", c.apiKey)
	}
	if c.baseURL != "http://example.com" {
		t.Errorf("expected baseURL=http://example.com, got %s", c.baseURL)
	}
}

func TestNewClientDefaultURL(t *testing.T) {
	// Given: no ORCA_API_URL set
	t.Setenv("ORCA_API_URL", "")

	// When
	c := NewClient()

	// Then
	if c.baseURL != "https://api.orcadb.ai" {
		t.Errorf("expected default base URL, got %s", c.baseURL)
	}
}

func TestNewClientTrimsTrailingSlash(t *testing.T) {
	c := NewClient(WithBaseURL("http://example.com/api/"))
	if c.baseURL != "http://example.com/api" {
		t.Errorf("expected trailing slash trimmed, got %s", c.baseURL)
	}
}

func TestApiKeyHeader(t *testing.T) {
	// Given
	var receivedKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedKey = r.Header.Get("Api-Key")
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("secret-123"), WithRetries(0))

	// When
	_, _ = c.get(context.Background(), "/test", nil)

	// Then
	if receivedKey != "secret-123" {
		t.Errorf("expected Api-Key=secret-123, got %s", receivedKey)
	}
}

func TestIsHealthy(t *testing.T) {
	// Given: healthy server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/check/healthy" {
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("test"), WithRetries(0))

	// When/Then
	if !c.IsHealthy(context.Background()) {
		t.Error("expected healthy=true")
	}
}

func TestIsHealthyFalse(t *testing.T) {
	// Given: unhealthy server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]bool{"ok": false})
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("test"), WithRetries(0))

	// When/Then
	if c.IsHealthy(context.Background()) {
		t.Error("expected healthy=false")
	}
}

func TestAPIErrorNotFound(t *testing.T) {
	// Given
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"detail": "resource not found"}`))
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("test"), WithRetries(0))

	// When
	_, err := c.get(context.Background(), "/missing", nil)

	// Then
	if !IsNotFound(err) {
		t.Errorf("expected IsNotFound, got: %v", err)
	}
}

func TestAPIErrorUnauthorized(t *testing.T) {
	// Given
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"detail": "Invalid API key"}`))
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("bad"), WithRetries(0))

	// When
	_, err := c.get(context.Background(), "/test", nil)

	// Then
	if !IsUnauthorized(err) {
		t.Errorf("expected IsUnauthorized, got: %v", err)
	}
}

func TestAPIErrorForbidden(t *testing.T) {
	// Given
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte(`{"reason": "insufficient permissions"}`))
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("test"), WithRetries(0))

	// When
	_, err := c.get(context.Background(), "/test", nil)

	// Then
	if !IsForbidden(err) {
		t.Errorf("expected IsForbidden, got: %v", err)
	}
}

func TestAPIErrorValidation(t *testing.T) {
	// Given
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		w.Write([]byte(`{"detail": [{"msg": "bad input", "loc": ["body"]}]}`))
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("test"), WithRetries(0))

	// When
	_, err := c.get(context.Background(), "/test", nil)

	// Then
	if !IsValidationError(err) {
		t.Errorf("expected IsValidationError, got: %v", err)
	}
}

func TestRetryOnTransientError(t *testing.T) {
	// Given: server fails once with 503 then succeeds
	attempt := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt == 1 {
			w.WriteHeader(503)
			w.Write([]byte(`{"detail": "unavailable"}`))
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("test"), WithRetries(2))
	c.baseDelay = time.Millisecond

	// When
	_, err := c.get(context.Background(), "/test", nil)

	// Then
	if err != nil {
		t.Errorf("expected success after retry, got: %v", err)
	}
	if attempt != 2 {
		t.Errorf("expected 2 attempts, got %d", attempt)
	}
}

func TestNoRetryWhenDisabled(t *testing.T) {
	// Given: server always returns 503
	attempt := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		w.WriteHeader(503)
		w.Write([]byte(`{"detail": "unavailable"}`))
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("test"), WithRetries(0))

	// When
	_, err := c.get(context.Background(), "/test", nil)

	// Then: only one attempt, returns error
	if err == nil {
		t.Fatal("expected error")
	}
	if attempt != 1 {
		t.Errorf("expected 1 attempt with retries disabled, got %d", attempt)
	}
}

func TestRetryExhausted(t *testing.T) {
	// Given: server always returns 429
	attempt := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		w.WriteHeader(429)
		w.Write([]byte(`{"detail": "rate limited"}`))
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("test"), WithRetries(2))
	c.baseDelay = time.Millisecond

	// When
	_, err := c.get(context.Background(), "/test", nil)

	// Then: 3 total attempts (1 initial + 2 retries), final attempt returns 429 error
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if attempt != 3 {
		t.Errorf("expected 3 attempts, got %d", attempt)
	}
	var apiErr *APIError
	if ok := errors.As(err, &apiErr); !ok || apiErr.StatusCode != 429 {
		t.Errorf("expected APIError with status 429, got: %v", err)
	}
}

func TestContextCancellation(t *testing.T) {
	// Given: a slow server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("test"), WithRetries(0))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// When
	_, err := c.get(ctx, "/slow", nil)

	// Then
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}
