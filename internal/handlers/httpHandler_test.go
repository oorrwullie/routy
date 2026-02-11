package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oorrwullie/routy/internal/models"
)

func TestCorsAllowedOrigin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		origin          string
		allowOrigins    []string
		allowCreds      bool
		wantOrigin      string
		wantVary        bool
	}{
		{
			name:       "no origin",
			origin:     "",
			allowOrigins: []string{"*"},
			wantOrigin: "",
			wantVary:   false,
		},
		{
			name:       "no allow list",
			origin:     "https://a.example",
			allowOrigins: nil,
			wantOrigin: "",
			wantVary:   false,
		},
		{
			name:       "wildcard without credentials",
			origin:     "https://a.example",
			allowOrigins: []string{"*"},
			wantOrigin: "*",
			wantVary:   false,
		},
		{
			name:       "wildcard with credentials",
			origin:     "https://a.example",
			allowOrigins: []string{"*"},
			allowCreds: true,
			wantOrigin: "https://a.example",
			wantVary:   true,
		},
		{
			name:       "explicit match",
			origin:     "https://a.example",
			allowOrigins: []string{"https://a.example"},
			wantOrigin: "https://a.example",
			wantVary:   true,
		},
		{
			name:       "no match",
			origin:     "https://a.example",
			allowOrigins: []string{"https://b.example"},
			wantOrigin: "",
			wantVary:   false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotOrigin, gotVary := corsAllowedOrigin(tt.origin, tt.allowOrigins, tt.allowCreds)
			if gotOrigin != tt.wantOrigin {
				t.Fatalf("origin mismatch: got %q want %q", gotOrigin, tt.wantOrigin)
			}
			if gotVary != tt.wantVary {
				t.Fatalf("vary mismatch: got %v want %v", gotVary, tt.wantVary)
			}
		})
	}
}

func TestApplyCORSHeaders(t *testing.T) {
	t.Parallel()

	cfg := &models.CORSConfig{
		AllowOrigins:    []string{"https://twh.org.ph"},
		AllowMethods:    []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:    []string{"Content-Type", "Authorization"},
		ExposeHeaders:   []string{"X-Request-Id"},
		AllowCredentials: true,
		MaxAge:          600,
	}

	req := httptest.NewRequest(http.MethodGet, "https://twh-example.pyrous.net/", nil)
	req.Header.Set("Origin", "https://twh.org.ph")
	rec := httptest.NewRecorder()

	applyCORSHeaders(rec, req, cfg)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://twh.org.ph" {
		t.Fatalf("allow-origin header mismatch: got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("allow-credentials header mismatch: got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, OPTIONS" {
		t.Fatalf("allow-methods header mismatch: got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "Content-Type, Authorization" {
		t.Fatalf("allow-headers header mismatch: got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Expose-Headers"); got != "X-Request-Id" {
		t.Fatalf("expose-headers header mismatch: got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Max-Age"); got != "600" {
		t.Fatalf("max-age header mismatch: got %q", got)
	}
	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("vary header mismatch: got %q", got)
	}
}

