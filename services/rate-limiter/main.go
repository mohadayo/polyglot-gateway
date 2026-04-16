package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

type RateLimiter struct {
	mu       sync.Mutex
	clients  map[string]*clientState
	limit    int
	windowMs int64
}

type clientState struct {
	tokens    []int64
}

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

type CheckResponse struct {
	Allowed   bool  `json:"allowed"`
	Remaining int   `json:"remaining"`
	Limit     int   `json:"limit"`
	ResetMs   int64 `json:"reset_ms"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type StatsResponse struct {
	ActiveClients int `json:"active_clients"`
	Limit         int `json:"limit"`
	WindowMs      int64 `json:"window_ms"`
}

func NewRateLimiter(limit int, windowMs int64) *RateLimiter {
	return &RateLimiter{
		clients:  make(map[string]*clientState),
		limit:    limit,
		windowMs: windowMs,
	}
}

func (rl *RateLimiter) Check(clientID string) CheckResponse {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now().UnixMilli()
	cutoff := now - rl.windowMs

	state, exists := rl.clients[clientID]
	if !exists {
		state = &clientState{}
		rl.clients[clientID] = state
	}

	// Remove expired tokens
	valid := make([]int64, 0, len(state.tokens))
	for _, t := range state.tokens {
		if t > cutoff {
			valid = append(valid, t)
		}
	}
	state.tokens = valid

	remaining := rl.limit - len(state.tokens)
	if remaining < 0 {
		remaining = 0
	}

	if len(state.tokens) >= rl.limit {
		resetMs := state.tokens[0] + rl.windowMs - now
		return CheckResponse{Allowed: false, Remaining: 0, Limit: rl.limit, ResetMs: resetMs}
	}

	state.tokens = append(state.tokens, now)
	remaining = rl.limit - len(state.tokens)

	return CheckResponse{Allowed: true, Remaining: remaining, Limit: rl.limit, ResetMs: rl.windowMs}
}

func (rl *RateLimiter) Stats() StatsResponse {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now().UnixMilli()
	cutoff := now - rl.windowMs
	active := 0
	for _, state := range rl.clients {
		for _, t := range state.tokens {
			if t > cutoff {
				active++
				break
			}
		}
	}
	return StatsResponse{ActiveClients: active, Limit: rl.limit, WindowMs: rl.windowMs}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func main() {
	port := getEnv("RATE_LIMITER_PORT", "8002")
	limitStr := getEnv("RATE_LIMIT", "100")
	windowStr := getEnv("RATE_WINDOW_MS", "60000")
	logLevel := getEnv("LOG_LEVEL", "INFO")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		log.Fatalf("Invalid RATE_LIMIT: %s", limitStr)
	}
	windowMs, err := strconv.ParseInt(windowStr, 10, 64)
	if err != nil {
		log.Fatalf("Invalid RATE_WINDOW_MS: %s", windowStr)
	}

	rl := NewRateLimiter(limit, windowMs)

	if logLevel == "DEBUG" {
		log.Println("Debug logging enabled")
	}

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonResponse(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
			return
		}
		log.Println("Health check requested")
		jsonResponse(w, http.StatusOK, HealthResponse{Status: "healthy", Service: "rate-limiter"})
	})

	http.HandleFunc("/check", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonResponse(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
			return
		}

		var body struct {
			ClientID string `json:"client_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ClientID == "" {
			log.Println("Check request with missing client_id")
			jsonResponse(w, http.StatusBadRequest, ErrorResponse{Error: "client_id is required"})
			return
		}

		result := rl.Check(body.ClientID)
		status := http.StatusOK
		if !result.Allowed {
			status = http.StatusTooManyRequests
			log.Printf("Rate limit exceeded for client: %s", body.ClientID)
		} else {
			log.Printf("Request allowed for client: %s (remaining: %d)", body.ClientID, result.Remaining)
		}
		jsonResponse(w, status, result)
	})

	http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonResponse(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
			return
		}
		stats := rl.Stats()
		log.Printf("Stats requested: %d active clients", stats.ActiveClients)
		jsonResponse(w, http.StatusOK, stats)
	})

	log.Printf("Starting rate-limiter on port %s (limit: %d, window: %dms)", port, limit, windowMs)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
