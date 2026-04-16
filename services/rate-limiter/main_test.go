package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func setupTestServer() *http.ServeMux {
	rl := NewRateLimiter(3, 60000)

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonResponse(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
			return
		}
		jsonResponse(w, http.StatusOK, HealthResponse{Status: "healthy", Service: "rate-limiter"})
	})

	mux.HandleFunc("/check", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonResponse(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
			return
		}
		var body struct {
			ClientID string `json:"client_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ClientID == "" {
			jsonResponse(w, http.StatusBadRequest, ErrorResponse{Error: "client_id is required"})
			return
		}
		result := rl.Check(body.ClientID)
		status := http.StatusOK
		if !result.Allowed {
			status = http.StatusTooManyRequests
		}
		jsonResponse(w, status, result)
	})

	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonResponse(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
			return
		}
		jsonResponse(w, http.StatusOK, rl.Stats())
	})

	return mux
}

func TestHealth(t *testing.T) {
	mux := setupTestServer()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp HealthResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Status != "healthy" {
		t.Fatalf("expected healthy, got %s", resp.Status)
	}
	if resp.Service != "rate-limiter" {
		t.Fatalf("expected rate-limiter, got %s", resp.Service)
	}
}

func TestCheckAllowed(t *testing.T) {
	mux := setupTestServer()
	body, _ := json.Marshal(map[string]string{"client_id": "test-client"})
	req := httptest.NewRequest(http.MethodPost, "/check", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp CheckResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Allowed {
		t.Fatal("expected allowed to be true")
	}
	if resp.Remaining != 2 {
		t.Fatalf("expected remaining 2, got %d", resp.Remaining)
	}
}

func TestCheckRateLimited(t *testing.T) {
	mux := setupTestServer()

	// Exhaust the limit (3 requests)
	for i := 0; i < 3; i++ {
		body, _ := json.Marshal(map[string]string{"client_id": "limited-client"})
		req := httptest.NewRequest(http.MethodPost, "/check", bytes.NewReader(body))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
	}

	// 4th request should be rate limited
	body, _ := json.Marshal(map[string]string{"client_id": "limited-client"})
	req := httptest.NewRequest(http.MethodPost, "/check", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
	var resp CheckResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Allowed {
		t.Fatal("expected allowed to be false")
	}
}

func TestCheckMissingClientID(t *testing.T) {
	mux := setupTestServer()
	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest(http.MethodPost, "/check", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCheckWrongMethod(t *testing.T) {
	mux := setupTestServer()
	req := httptest.NewRequest(http.MethodGet, "/check", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestStats(t *testing.T) {
	mux := setupTestServer()

	// Make a request first
	body, _ := json.Marshal(map[string]string{"client_id": "stats-client"})
	req := httptest.NewRequest(http.MethodPost, "/check", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Check stats
	req = httptest.NewRequest(http.MethodGet, "/stats", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp StatsResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.ActiveClients != 1 {
		t.Fatalf("expected 1 active client, got %d", resp.ActiveClients)
	}
	if resp.Limit != 3 {
		t.Fatalf("expected limit 3, got %d", resp.Limit)
	}
}

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(10, 30000)
	if rl.limit != 10 {
		t.Fatalf("expected limit 10, got %d", rl.limit)
	}
	if rl.windowMs != 30000 {
		t.Fatalf("expected window 30000, got %d", rl.windowMs)
	}
}

func TestIndependentClients(t *testing.T) {
	mux := setupTestServer()

	// Client A uses 3 requests (limit)
	for i := 0; i < 3; i++ {
		body, _ := json.Marshal(map[string]string{"client_id": "client-a"})
		req := httptest.NewRequest(http.MethodPost, "/check", bytes.NewReader(body))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
	}

	// Client B should still be allowed
	body, _ := json.Marshal(map[string]string{"client_id": "client-b"})
	req := httptest.NewRequest(http.MethodPost, "/check", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp CheckResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Allowed {
		t.Fatal("Client B should be allowed independently of Client A")
	}
}
