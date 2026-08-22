package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDoRequest(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		type response struct {
			Message string `json:"message"`
			Count   int    `json:"count"`
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Content-Type") != "application/json" {
				t.Error("missing Content-Type header")
			}
			if r.Header.Get("X-Custom") != "test-value" {
				t.Error("missing custom header")
			}
			if r.Method != "POST" {
				t.Errorf("method = %s, want POST", r.Method)
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response{Message: "ok", Count: 42})
		}))
		defer server.Close()

		var result response
		err := doRequest(
			context.Background(),
			"POST",
			server.URL,
			map[string]string{"key": "value"},
			map[string]string{"X-Custom": "test-value"},
			&result,
		)

		if err != nil {
			t.Fatalf("doRequest failed: %v", err)
		}
		if result.Message != "ok" {
			t.Errorf("Message = %q, want %q", result.Message, "ok")
		}
		if result.Count != 42 {
			t.Errorf("Count = %d, want 42", result.Count)
		}
	})

	t.Run("server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("not json"))
		}))
		defer server.Close()

		var result map[string]any
		err := doRequest(context.Background(), "POST", server.URL, nil, nil, &result)
		if err == nil {
			t.Fatal("expected error for invalid JSON response")
		}
		want := "API error (HTTP 500): not json"
		if err.Error() != want {
			t.Errorf("err = %q, want %q", err.Error(), want)
		}
	})

	t.Run("http status with JSON error body is unwrapped", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]string{"message": "invalid api key"},
			})
		}))
		defer server.Close()

		var result map[string]any
		err := doRequest(context.Background(), "POST", server.URL, nil, nil, &result)
		if err == nil {
			t.Fatal("expected error for 401 response")
		}
		want := "API error (HTTP 401): invalid api key"
		if err.Error() != want {
			t.Errorf("err = %q, want %q", err.Error(), want)
		}
	})

	t.Run("http status with HTML body yields snippet", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			w.Write([]byte("<html><body>Bad Gateway</body></html>"))
		}))
		defer server.Close()

		var result map[string]any
		err := doRequest(context.Background(), "POST", server.URL, nil, nil, &result)
		if err == nil {
			t.Fatal("expected error for 502 response")
		}
		want := "API error (HTTP 502): <html><body>Bad Gateway</body></html>"
		if err.Error() != want {
			t.Errorf("err = %q, want %q", err.Error(), want)
		}
	})

	t.Run("long non-JSON error bodies are truncated", func(t *testing.T) {
		long := strings.Repeat("x", 5000)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(long))
		}))
		defer server.Close()

		var result map[string]any
		err := doRequest(context.Background(), "POST", server.URL, nil, nil, &result)
		if err == nil {
			t.Fatal("expected error for 503 response")
		}
		if len(err.Error()) > 300 || !strings.Contains(err.Error(), "HTTP 503") {
			t.Errorf("err = %q, want short snippet mentioning HTTP 503", err.Error())
		}
	})

	t.Run("invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("{invalid"))
		}))
		defer server.Close()

		var result map[string]any
		err := doRequest(context.Background(), "POST", server.URL, nil, nil, &result)
		if err == nil {
			t.Error("expected error for malformed JSON")
		}
	})
}

func TestDoStream(t *testing.T) {
	t.Run("returns open response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				t.Errorf("method = %s, want POST", r.Method)
			}
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("data: hello\n\n"))
		}))
		defer server.Close()

		resp, err := doStream(
			context.Background(),
			server.URL,
			map[string]string{"key": "value"},
			map[string]string{"X-Custom": "test"},
		)
		if err != nil {
			t.Fatalf("doStream failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
	})

	t.Run("nil headers", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		resp, err := doStream(context.Background(), server.URL, nil, nil)
		if err != nil {
			t.Fatalf("doStream with nil headers failed: %v", err)
		}
		resp.Body.Close()
	})
}
